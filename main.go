package main

import (
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"wphpfpm/phpfpm"
	"wphpfpm/server"

	"github.com/chai2010/winsvc"
	"gopkg.in/alecthomas/kingpin.v2"
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
		checkFlagConfigFile(flagConfigFile)

		log.Println(serviceName, "Run service")
		if err := winsvc.RunAsService(serviceName, startService, stopService, false); err != nil {
			log.Fatalf(serviceName+" run: %v\n", err)
		}
	} else {
		// command line mode
		initCommandFlag()
		switch kingpin.Parse() {
		case commandInstall.FullCommand():
			checkFlagConfigFile(flagConfigFile)
			installService()
		case commandUninstall.FullCommand():
			if err := winsvc.RemoveService(serviceName); err != nil {
				log.Fatalln("Uninstall service: ", err)
			}
			log.Println("Uninstall service: success")
		case commandRun.FullCommand():
			checkFlagConfigFile(flagConfigFile)
			log.Println("Start in console mode , press CTRL+C to exited ...")
			startService()
		case commandStart.FullCommand():
			if err := winsvc.StartService(serviceName); err != nil {
				log.Fatalln("startService:", err)
			}
			log.Println("Start service: success")
		case commandStop.FullCommand():
			if err := winsvc.StopService(serviceName); err != nil {
				log.Fatalln("Stop service: ", err)
			}
			log.Println("Stop service: success")
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

func checkFlagConfigFile(configFile *string) {
	if _, err := os.Stat(*configFile); err != nil {
		if os.IsNotExist(err) {
			log.Fatalf("Config file %s is not exist.", *configFile)
		}
	}
}

// 安裝服務
func installService() {
	var serviceExec string
	var err error

	if serviceExec, err = winsvc.GetAppPath(); err != nil {
		log.Fatal(err)
	}
	if err := os.Chdir(filepath.Dir(serviceExec)); err != nil {
		log.Fatal(err)
	}

	abs, err := filepath.Abs(*flagConfigFile)

	serviceExecFull := "\"" + serviceExec + "\"" + " --conf=" + abs
	args := []string{"--conf", abs}
	log.Printf("Service install name : %s , binpath : %s\n", serviceName, serviceExecFull)
	if err := winsvc.InstallService(serviceExec, serviceName, serviceDesc, args...); err != nil {
		log.Fatalf("Install service : (%s, %s): %v\n", serviceName, serviceDesc, err)
		log.Fatalf("Install service : error.\n")
	}

	log.Println("Install service : success.")
}

// 啟動服務
func startService() {

	err := phpfpm.InitConfig(*flagConfigFile)
	if err != nil {
		log.Fatalln("startService:", err)
	}

	phpfpm.Start()

	log.Println("PHP-CGI Manage started")

	var events server.Event

	events.OnConnect = func(c *server.Conn) (action server.Action) {
		p := phpfpm.GetIdleProcess(c.Server().Tag.(int))
		if p == nil {
			log.Printf("Can not get php-cgi process\n")
			action = server.Close
			return
		}
		err := p.Proxy(c) // blocked
		if err != nil {
			log.Printf("proxy error : %s\n", err.Error())
		}
		phpfpm.PutIdleProcess(p)

		return
	}

	events.OnDisconnect = func(c *server.Conn) (action server.Action) {
		log.Println("On Disconnect")
		return
	}

	conf := phpfpm.Conf()

	var wg sync.WaitGroup

	wg.Add(len(conf.Instances))

	servers = make([]*server.Server, len(conf.Instances))

	for i := 0; i < len(conf.Instances); i++ {
		instance := conf.Instances[i]
		servers[i] = &server.Server{MaxConnections: instance.MaxProcesses, BindAddress: instance.Bind, Tag: i}

		log.Printf("Start server #%d on %s\n", i, servers[i].BindAddress)

		go func(s *server.Server) {
			err := s.Serve(events)
			if err != nil {
				log.Printf("Service serve error : %s\n", err.Error())
			}
			wg.Done()
		}(servers[i])
	}
	log.Println("Service Startted")

	// 這段處理 CTRL + C
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for sig := range c {
			println(sig.String())
			stopService()
			return
		}
	}()

	wg.Wait()
	phpfpm.Stop()
	log.Println("Service Stopped")
}

// 停止服務
func stopService() {

	for i := 0; i < len(servers); i++ {
		servers[i].Shutdown()
	}

}
