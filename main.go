package main

import (
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"time"
	"wphpfpm/phpfpm"

	"github.com/chai2010/winsvc"
	"github.com/tidwall/evio"
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

	shutdownEvioServer = false
)

type evioContext struct {
	inStream *evio.InputStream
	process  *phpfpm.Process
}

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

func startServe() {

}

// 啟動服務
func startService() {

	err := phpfpm.InitConfig(*flagConfigFile)
	if err != nil {
		log.Fatalln("startService:", err)
	}

	phpfpm.Start()

	log.Println("PHP-CGI Manage started")

	// 這段處理 CTRL + C
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for sig := range c {
			println(sig.String())
			stopService()
		}
	}()

	var events evio.Events

	events.Opened = func(c evio.Conn) (out []byte, opts evio.Options, action evio.Action) {
		// c.SetContext(&evio.InputStream{})
		log.Printf("opened: laddr: %v , raddr: %v , addrIndex : %d", c.LocalAddr(), c.RemoteAddr(), c.AddrIndex())
		var p *phpfpm.Process
		for {
			p = phpfpm.GetIdleProcess(c.AddrIndex())
			if p != nil {
				break
			}
			// no idle process , wait
			time.Sleep(time.Duration(time.Millisecond * 100))
		}

		err = phpfpm.SetProcessActive(p, true)
		if err != nil {
			log.Println(err)
			action = evio.Close
			return
		}
		log.Println("Set idle process to active")

		c.SetContext(p)
		p.StartWaitResponse(c)
		return
	}

	// f, err := os.OpenFile("d:\\test\\wphpfp.bin", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	// defer f.Close()

	events.Data = func(c evio.Conn, in []byte) (out []byte, action evio.Action) {
		log.Println("event data")
		p := c.Context().(*phpfpm.Process)
		if in != nil {
			// f.WriteString("CADDY IN\n\n")
			// f.Write(in)
			p.Write(in)
			log.Println("Write to php-cgi success")
		} else {
			out = p.Response()
		}

		return
	}
	events.Closed = func(c evio.Conn, err error) (action evio.Action) {
		log.Println("event closed")
		p := c.Context().(*phpfpm.Process)
		// 設定 php-cgi process 可以不用了
		phpfpm.SetProcessActive(p, false)
		log.Printf("closed: laddr: %v: raddr: %v", c.LocalAddr(), c.RemoteAddr())
		return
	}

	events.Tick = func() (delay time.Duration, action evio.Action) {
		// 這段判斷是否要停止服務 , 每秒判斷一次
		delay = time.Second
		if shutdownEvioServer {
			phpfpm.Stop()
			action = evio.Shutdown
			log.Println("Tick shutdown")
		}
		return
	}
	log.Println("Service started.")

	if err := evio.Serve(events, "tcp://127.0.0.1:8000"); err != nil {
		log.Fatalln(err)
	}
	log.Println("Service Stopped")
}

// 停止服務
func stopService() {
	shutdownEvioServer = true
}
