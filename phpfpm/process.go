package phpfpm

import (
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	log "github.com/sirupsen/logrus"
	"gopkg.in/natefinch/npipe.v2"
)

// Process : struct
type Process struct {
	execPath   string
	args       []string
	env        []string
	cmd        *exec.Cmd
	pipe       *npipe.PipeConn
	pippedName string // php-cgi 執行時指定的 pipped name

	requestCount int // 紀錄當前執行中的 php-cgi 已經接受幾次要求了

	restartChan chan bool

	copyRbuf           []byte
	copyWbuf           []byte
	execWithPippedName string

	wg sync.WaitGroup

	inUse     bool  // 进程正在工作
	inPool    bool  // 创建进程时是否已加入到进程池
	closed    bool  // 进程已退出
	idleAt    int64 // 空闲开始时间
}

var (
	namedPipeNumber      = time.Now().Unix()
	namedPipeNumberMutex sync.Mutex
)

// newProcess : Create new Process
// 建立一個新的 Process
func newProcess(execPath string, args []string, env []string) *Process {
	p := new(Process)
	p.execPath = execPath
	p.args = args
	p.env = env
	p.restartChan = make(chan bool)
	p.copyRbuf = make([]byte, 4096)
	p.copyWbuf = make([]byte, 16384)
	return p
}

// IdleAt time of turned to idle status
func (p *Process) IdleAt() time.Time {
	unix := atomic.LoadInt64(&p.idleAt)
	return time.Unix(unix, 0)
}

// TouchIdleTime set idle time as now
func (p *Process) TouchIdleTime() {
	atomic.StoreInt64(&p.idleAt, time.Now().Unix())
	p.inUse = false
}

// TryStart will execute php-cgi twince
func (p *Process) TryStart() (err error) {
	// pippedName 是啟動 php-cgi 時候指定 -b pipename 使用的
	namedPipeNumberMutex.Lock()
	namedPipeNumber++
	namedPipeNumberMutex.Unlock()
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
// Proxy 將 tcp 來源跟 windows named pipe 直接做讀寫
// 返回值 serr 代表由 http server 讀取資料寫至 php-cgi 的錯誤
// 返回值 terr 代表由 php-cgi 讀取資料寫至 http server 的錯誤
func (p *Process) Proxy(conn net.Conn) (serr error, terr error) {

	terr = p.connectPipe()
	if terr != nil {
		return
	}

	p.wg.Add(2)
	go func() {
		// read from web server , write to php-cgi
		_, serr = io.CopyBuffer(p.pipe, conn, p.copyRbuf)
		p.wg.Done()
	}()
	go func() {
		// read from php-cgi , write to web server
		_, terr = io.CopyBuffer(conn, p.pipe, p.copyWbuf)
		p.wg.Done()
	}()

	p.wg.Wait()
	p.pipe.Close()
	p.pipe = nil
	return
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
