package phpfpm

import (
	"container/list"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"

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
}

var (
	namedPipeNumber = time.Now().Unix()
)

// NewProcess : Create new PhpProcess
// 建立一個新的 PhpProcess
func newProcess(execPath string, args []string, env []string) *Process {
	p := new(Process)
	p.execPath = execPath
	p.args = args
	p.env = env
	p.restartChan = make(chan bool)
	return p
}

// TryStart : 會嘗試先啟動 php-cgi , 若啟動兩次失敗，會返回錯誤
func (p *Process) TryStart() (err error) {
	// pippedName 是啟動 php-cgi 時候指定 -b pipename 使用的
	namedPipeNumber++
	p.pippedName = `\\.\pipe\wphpfpm\wphpfpm.` + strconv.FormatInt(namedPipeNumber, 10)
	p.requestCount = 0
	for i := 0; i < 2; i++ {
		args := append(p.args, "-b", p.pippedName)
		p.cmd = nil
		p.cmd = exec.Command(p.execPath, args...)
		p.cmd.Env = os.Environ()
		p.cmd.Env = append(p.cmd.Env, p.env...)
		err = p.cmd.Start()
		if err == nil {
			i = 3
		}
	}
	if err != nil {
		log.Printf("Start php-cgi error : %s\n", err.Error())
	}
	return
}

// connectPipe : 連接 pipe
func (p *Process) connectPipe() error {
	var err error
	if p.pipe != nil {
		p.pipe.Close()
	}

	p.pipe, err = npipe.Dial(p.pippedName)
	if err != nil {
		log.Println("connectPipe() err " + err.Error())
		return err
	}
	p.requestCount++
	log.Printf("connectPipe() requestCount : %d ,  success %s", p.requestCount, p.pippedName)

	return nil
}

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
		_, err := io.Copy(conn, p.pipe)
		if err != nil {
			retErr = err
		}

		wg.Done()
	}()

	go func() {
		_, err := io.Copy(p.pipe, conn)
		if err != nil {
			retErr = err
		}

		wg.Done()
	}()

	wg.Wait()

	return retErr
}

// Kill : 停止 php-cgi
func (p *Process) Kill() error {
	log.Printf("Process %p killing ...\n", p)
	return p.cmd.Process.Kill()
	// return p.cmd.Process.Signal(os.Interrupt)
}
