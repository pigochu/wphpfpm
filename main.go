package main

import (
	"bytes"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"wphpfpm/conf"
	"wphpfpm/phpfpm"
	"wphpfpm/server"

	"github.com/chai2010/winsvc"
	log "github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	serviceName      = "wphpfpm"
	serviceDesc      = "PHP FastCGI Manager for windows"
	app              = kingpin.New(serviceName, serviceDesc)
	commandInstall   *kingpin.CmdClause
	commandUninstall *kingpin.CmdClause
	commandStart     *kingpin.CmdClause
	commandStop      *kingpin.CmdClause
	commandRun       *kingpin.CmdClause
	flagConfigFile   *string

	servers []*server.Server
)

func main() {
	if !winsvc.IsAnInteractiveSession() {
		// run as service
		flag := kingpin.Flag("conf", "Config file path , required by install or run.")
		flagConfigFile = flag.Required().String()
		kingpin.Parse()
		checkConfigFileExist(*flagConfigFile)

		fmt.Println(serviceName, "Run service")
		if err := winsvc.RunAsService(serviceName, startService, stopService, false); err != nil {
			log.Fatalf(serviceName+" run: %v\n", err)
		}
	} else {
		// command line mode
		initCommandFlag()
		switch kingpin.Parse() {
		case commandInstall.FullCommand():
			checkConfigFileExist(*flagConfigFile)
			installService()
		case commandUninstall.FullCommand():
			if err := winsvc.RemoveService(serviceName); err != nil {
				fmt.Println("Uninstall service: ", err)
				os.Exit(1)
			}
			log.Println("Uninstall service: success")
		case commandRun.FullCommand():
			checkConfigFileExist(*flagConfigFile)
			startService()
		case commandStart.FullCommand():
			if err := winsvc.StartService(serviceName); err != nil {
				fmt.Println("Start service:", err)
				os.Exit(1)
			}
			fmt.Println("Start service: success")
		case commandStop.FullCommand():
			if err := winsvc.StopService(serviceName); err != nil {
				fmt.Println("Stop service:", err)
				os.Exit(1)
			}
			fmt.Println("Stop service: success")
			return
		}
	}

}

func initCommandFlag() {
	commandInstall = kingpin.Command("install", "Install as service")
	commandUninstall = kingpin.Command("uninstall", "Uninstall service")
	commandStart = kingpin.Command("start", "Start service.")
	commandStop = kingpin.Command("stop", "Stop service.")
	commandRun = kingpin.Command("run", "Run in console mode")
	flag := kingpin.Flag("conf", "Config file path , required by install or run.")
	if len(os.Args) > 1 && (os.Args[1] == "install" || os.Args[1] == "run") {
		flagConfigFile = flag.Required().String()
	} else {
		flagConfigFile = flag.String()
	}
}

// 安裝服務
func installService() {
	var serviceExec string
	var err error

	if serviceExec, err = winsvc.GetAppPath(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if err := os.Chdir(filepath.Dir(serviceExec)); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	abs, err := filepath.Abs(*flagConfigFile)

	serviceExecFull := "\"" + serviceExec + "\"" + " --conf=" + abs
	args := []string{"--conf", abs}
	fmt.Printf("Service install name : %s , binpath : %s\n", serviceName, serviceExecFull)
	if err := winsvc.InstallService(serviceExec, serviceName, serviceDesc, args...); err != nil {
		fmt.Printf("Install service error : %s\n", err.Error())
		os.Exit(1)
	}

	fmt.Println("Install service successfully.")
}

// 啟動服務
func startService() {
	config, err := conf.LoadFile(*flagConfigFile)

	if err != nil {
		fmt.Printf("Config load error : %s\n", err.Error())
		os.Exit(1)
	}

	fmt.Fprintf(os.Stdout, "Start in console mode , press CTRL+C to exit ...\r\n\r\n")
	initLogger(config)
	err = phpfpm.Start(config)
	if err != nil {
		log.Fatalf("Can not start service : %s\n", err.Error())
	}

	var events server.Event

	events.OnConnect = func(c *server.Conn) (action server.Action) {

		p := phpfpm.GetIdleProcess(c.Server().Tag.(int))

		if p == nil {
			if log.IsLevelEnabled(log.ErrorLevel) {
				log.Error("Can not get php-cgi process")
			}
			action = server.Close
			return
		}
		serr, terr := p.Proxy(c) // blocked
		if log.IsLevelEnabled(log.DebugLevel) && (serr != nil || terr != nil) {
			log.Debugf("php-cgi(%s) proxy error , serr : %s , terr: %s", p.ExecWithPippedName(), serr, terr)
		}
		phpfpm.PutIdleProcess(p)

		return
	}

	conf := phpfpm.Conf()

	var wg sync.WaitGroup

	wg.Add(len(conf.Instances))

	servers = make([]*server.Server, len(conf.Instances))

	for i := 0; i < len(conf.Instances); i++ {
		instance := conf.Instances[i]
		servers[i] = &server.Server{MaxConnections: instance.MaxProcesses, BindAddress: instance.Bind, Tag: i}

		log.Infof("Start server #%d on %s", i, servers[i].BindAddress)

		go func(s *server.Server) {
			err := s.Serve(events)
			if err != nil {
				log.Errorf("Service serve error : %s", err.Error())
			}
			wg.Done()
		}(servers[i])
	}
	log.Info("Service running ...")

	// 這段處理 CTRL + C
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for sig := range c {
			log.Infof("Service got signal: %s", sig.String())
			stopService()
			return
		}
	}()

	wg.Wait()
	phpfpm.Stop()
	log.Info("Service Stopped.")
}

// 停止服務
func stopService() {

	for i := 0; i < len(servers); i++ {
		servers[i].Shutdown()
	}

}

func checkConfigFileExist(filepath string) {
	exist := conf.FileExist(filepath)
	if !exist {
		fmt.Printf("Could not load config file : %s", filepath)
		os.Exit(1)
	}
}

func initLogger(config *conf.Conf) {

	formatter := &MyTextFormatter{timeFormat: "2006-01-02 15:04:05 -0700"}
	log.SetFormatter(formatter)

	// Set logger
	if len(config.Logger.Filename) > 0 {

		logDir := filepath.Dir(config.Logger.Filename)
		exeDir, err := filepath.Abs(filepath.Dir(os.Args[0])) // 執行檔的路徑
		if err != nil {
			log.Fatal(err)
		}

		if logDir == "." || logDir == "" {
			// 如果 Filename 沒指定路徑，修正為 exe 的路徑
			config.Logger.Filename = exeDir + "\\" + config.Logger.Filename
		}

		logger := &lumberjack.Logger{
			Filename:   config.Logger.Filename,
			MaxSize:    config.Logger.MaxSize,
			MaxBackups: config.Logger.MaxBackups,
			MaxAge:     config.Logger.MaxAge,
			Compress:   config.Logger.Compress,
		}
		log.SetOutput(logger)
		fmt.Fprintf(os.Stdout, "Logger ouput set to file %s .\n", config.Logger.Filename)
	} else {
		fmt.Fprintf(os.Stdout, "Logger ouput set to console.\n")
	}

	if config.LogLevel == "" {
		config.LogLevel = "ERROR"
	}

	logLevel, err := log.ParseLevel(config.LogLevel)
	if err != nil {
		log.Fatalf("LogLevel %s can not parse.", config.LogLevel)
	}
	log.SetLevel(logLevel)
	log.Infof("Set LogLevel to %s.", strings.ToUpper(logLevel.String()))

	// Repair config
	for i := 0; i < len(config.Instances); i++ {
		if config.Instances[i].MaxRequestsPerProcess < 1 {
			// Repair MaxRequestsPerProcess
			log.Warnf("Instance #%d MaxRequestsPerProcess is less 1 , set to 500", i)
			config.Instances[i].MaxRequestsPerProcess = 500
		}

		if config.Instances[i].MaxProcesses < 1 {
			//Repair MaxProcesses
			log.Warnf("Instance #%d MaxProcesses is less 1 , set to 4", i)
			config.Instances[i].MaxProcesses = 4
		}
	}
}

// MyTextFormatter logrus custom formatter
type MyTextFormatter struct {
	timeFormat string
}

// Format logrus custom format
func (f *MyTextFormatter) Format(entry *log.Entry) ([]byte, error) {
	var b *bytes.Buffer
	if entry.Buffer != nil {
		b = entry.Buffer
	} else {
		b = &bytes.Buffer{}
	}
	b.WriteString(entry.Time.Format(f.timeFormat))
	b.WriteString(" [")
	b.WriteString(entry.Level.String())
	b.WriteString("]: ")
	b.WriteString(entry.Message)
	b.WriteByte('\n')
	return b.Bytes(), nil
}
