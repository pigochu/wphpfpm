package server

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

const (
	ReloadCmd  = svc.Cmd(129) // 128-255 user-defined control code
	RestartCmd = svc.Cmd(130)
)

// ReloadService send user defined signal to service
func ReloadService(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("svc.controlService: could not access service: %v", err)
	}
	defer s.Close()

	_, err = s.Control(ReloadCmd)
	if err != nil {
		return fmt.Errorf("svc.controlService: could not send control=ReloadCmd: %v", err)
	}
	return nil
}

// RunAsService start service
func RunAsService(name string, service svc.Handler) (err error) {

	log.Infof("svc.RunAsService: starting %s service", name)
	if err = svc.Run(name, service); err != nil {
		log.Errorf("%s service failed: %v", name, err)
		return
	}
	log.Infof("svc.RunAsService: %s service stopped", name)
	return
}

// WinService service handler
type WinService struct {
	Start  func()
	Stop   func()
	Reload func()
	Reopen func()
}

// Execute service executer
func (p *WinService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	log.Info("svc.Execute:" + "begin")
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown | svc.AcceptPauseAndContinue
	changes <- svc.Status{State: svc.StartPending}
	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

	go p.Start()

loop:
	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
				// testing deadlock from https://code.google.com/p/winsvc/issues/detail?id=4
				time.Sleep(100 * time.Millisecond)
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				break loop
			case svc.Pause:
				changes <- svc.Status{State: svc.Paused, Accepts: cmdsAccepted}
			case svc.Continue:
				changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
			case ReloadCmd, RestartCmd:
				if p.Reload != nil {
					p.Reload()
					log.Errorf("svc.Execute:: Reload()")
				}
				changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
			default:
				log.Errorf("svc.Execute:: unexpected control request #%d", c)
			}
		}
	}
	changes <- svc.Status{State: svc.StopPending}
	p.Stop()

	log.Info("svc.Execute:" + "end")
	return
}
