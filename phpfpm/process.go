package phpfpm

import (
	"container/list"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"gopkg.in/natefinch/npipe.v2"
)

// Instance : struct
type Instance struct {
	execPath  string
	args      []string
	env       []string
	processes []Process
}

// Process : struct
type Process struct {
	execPath   string
	args       []string
	env        []string
	cmd        *exec.Cmd
	mapIndex   int // 這個是在 phpfpm.go 中的 idleprocess or activeprocess 連結用的
	mapElement *list.Element
	pipe       *npipe.PipeConn
	pippedName string // php-cgi 執行時指定的 pipped name

	requestCount int // 紀錄當前執行中的 php-cgi 已經接受幾次要求了

	// 這個是要標記是否 server 要關機了，當 kill process 才不會再啟動
	serviceShutdown bool

	restartChan chan bool

	copyRbuf           []byte
	copyWbuf           []byte
	execWithPippedName string
}

var (
	namedPipeNumber = time.Now().Unix()
)

// newProcess : Create new Process
// 建立一個新的 Process
func newProcess(execPath string, args []string, env []string) *Process {
	p := new(Process)
	p.execPath = execPath
	p.args = args
	p.env = env
	p.restartChan = make(chan bool)
	p.copyRbuf = make([]byte, 8192)
	p.copyWbuf = make([]byte, 8192)
	return p
}

// TryStart will execute php-cgi twince
func (p *Process) TryStart() (err error) {
	// pippedName 是啟動 php-cgi 時候指定 -b pipename 使用的
	namedPipeNumber++
	p.pippedName = `\\.\pipe\wphpfpm\wphpfpm.` + strconv.FormatInt(namedPipeNumber, 10)
	p.requestCount = 0
	p.execWithPippedName = p.execPath + " -> " + p.pippedName

	log.Debugf("Trying to start php-cgi(%s).", p.execWithPippedName)
	for i := 0; i < 2; i++ {
		args := append(p.args, "-b", p.pippedName)
		p.cmd = nil
		p.cmd = exec.Command(p.execPath, args...)
		p.cmd.Env = os.Environ()
		p.cmd.Env = append(p.cmd.Env, p.env...)
		err = p.cmd.Start()
		if err == nil {
			i = 3
			if log.IsLevelEnabled(log.DebugLevel) {
				log.Debugf("php-cgi(%s) executing now.", p.execWithPippedName)
			}
		}
	}

	if err != nil {
		log.Errorf("php-cgi(%s) can not start , because %s", p.execWithPippedName, err.Error())
	}
	return
}

// connectPipe will connect to php-cgi named pipe
func (p *Process) connectPipe() error {
	var err error
	//if p.pipe != nil {
	//		p.pipe.Close()
	//}

	p.pipe, err = npipe.Dial(p.pippedName)
	if err != nil {
		log.Errorf("Connect to php-cgi(%s) error , because %s", p.execWithPippedName, err.Error())
		return err
	}
	p.requestCount++
	if log.IsLevelEnabled(log.DebugLevel) {
		log.Debugf("Connect to php-cgi(%s) successfully.", p.execWithPippedName)
	}

	return nil
}

// Proxy net.Conn <> Windows-named-pipe
// Proxy 將 tcp 來源跟 windows named pipe 直接做讀寫 , 完成時返回 nil
func (p *Process) Proxy(conn net.Conn) error {
	var retErr error

	err := p.connectPipe()
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		_, err := io.CopyBuffer(conn, p.pipe, p.copyRbuf)
		if err != nil {
			retErr = err
		}

		wg.Done()
	}()

	go func() {
		_, err := io.CopyBuffer(p.pipe, conn, p.copyWbuf)
		if err != nil {
			retErr = err
		}

		wg.Done()
	}()

	wg.Wait()
	p.pipe.Close()
	p.pipe = nil
	return retErr
}

// Kill php-cgi process
func (p *Process) Kill() (err error) {
	err = p.cmd.Process.Kill()

	if err != nil {
		if log.IsLevelEnabled(log.ErrorLevel) {
			log.Errorf("Kill php-cgi(%s) error , because %s", p.execWithPippedName, err.Error())
		}
	} else {
		if log.IsLevelEnabled(log.DebugLevel) {
			log.Debugf("Kill php-cgi(%s) successfully.", p.execWithPippedName)
		}
	}

	return
}

// ExecWithPippedName ...
func (p *Process) ExecWithPippedName() string {
	return p.execWithPippedName
}
