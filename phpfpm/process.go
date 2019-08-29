package phpfpm

import (
	"container/list"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/tidwall/evio"
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
	execPath     string
	args         []string
	env          []string
	cmd          *exec.Cmd
	mapIndex     int // 這個是在 phpfpm.go 中的 idleprocess or activeprocess 連結用的
	mapElement   *list.Element
	pipe         *npipe.PipeConn
	pippedName   string // php-cgi 執行時指定的 pipped name
	responseData []byte // php-cgi 返回的資料
	requestCount int    // 紀錄當前執行中的 php-cgi 已經接受幾次要求了

	pipeMutex sync.Mutex
	// 這個是要標記是否 server 要關機了，當 kill process 才不會再啟動
	serviceShutdown bool
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
	return p
}

// TryStart : 會嘗試先啟動 php-cgi , 若啟動兩次失敗，會返回錯誤
func (p *Process) TryStart() error {
	var err error
	// pippedName 是啟動 php-cgi 時候指定 -b pipename 使用的
	namedPipeNumber++
	p.pippedName = `\\.\pipe\wphpfpm\wphpfpm.` + strconv.FormatInt(namedPipeNumber, 10)

	for i := 0; i < 2; i++ {
		p.args = append(p.args, "-b", p.pippedName)
		p.cmd = exec.Command(p.execPath, p.args...)
		p.cmd.Env = os.Environ()
		p.cmd.Env = append(p.cmd.Env, p.env...)
		err = p.cmd.Start()
		if err == nil {
			p.requestCount = 0
			return nil
		}
	}
	return err
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

// StartWaitResponse ..
func (p *Process) StartWaitResponse(c evio.Conn) error {

	errroot := p.connectPipe()
	if errroot != nil {
		return errroot
	}

	// background read pipe
	go func() {
		var err error
		p.responseData, err = ioutil.ReadAll(p.pipe)
		if len(p.responseData) > 0 {
			// notify evio onData
			c.Wake()
		}
		if err != nil {
			log.Println("background read pipe err " + err.Error())
		}
	}()

	return nil
}

// Write ...
func (p *Process) Write(data []byte) (int, error) {
	return p.pipe.Write(data)
}

// Response ...
func (p *Process) Response() []byte {
	return p.responseData
}

// Kill : 停止 php-cgi
func (p *Process) Kill() error {
	log.Printf("Process %p killing ...\n", p)
	return p.cmd.Process.Kill()
	// return p.cmd.Process.Signal(os.Interrupt)
}
