package phpfpm

import (
	"sync"
	"time"
	"wphpfpm/conf"

	log "github.com/sirupsen/logrus"
)

// Instance : struct
type Instance struct {
	// execPath string
	// args     []string
	// env      []string
	// processes []Process
	idleProcesses []*Process // php-cgi 如果沒有任何連線處理，都存在這
	processes     []*Process
	stopManage    bool // 如果調用 Stop() , 這個會是 true , 同時 mon() 也不會繼續監控
	mutex         sync.Mutex

	conf          conf.Instance
	instanceIndex int
}

// NewPhpFpmInstance New PhpFpm Instance
func NewPhpFpmInstance(instanceIndex int, conf conf.Instance) *Instance {
	return &Instance{
		conf:          conf,
		instanceIndex: instanceIndex,
	}
}

// Start php-cgi manager
func (inst *Instance) Start() (err error) {
	log.Info("phpfpm starting.")
	inst.idleProcesses = make([]*Process, 0, inst.conf.MaxProcesses)
	inst.processes = make([]*Process, 0, inst.conf.MaxProcesses)

	for j := 0; j < inst.conf.MinProcesses; j++ {
		err := inst.createAndPushProcess()
		if err != nil {
			inst.Stop()
			return err
		}
	}

	// 回收多余的空闲进程
	if inst.conf.IdleTimeout > 0 {
		go inst.release(time.Duration(inst.conf.IdleTimeout) * time.Second)
	}

	log.Info("phpfpm is in loop.")
	return
}

func (inst *Instance) createAndPushProcess() (err error) {
	if inst.stopManage {
		return
	}

	p1 := newProcess(inst.conf.ExecPath, inst.conf.Args, inst.conf.Env)

	log.Infof("Starting php-cgi(%s)", p1.ExecWithPippedName())

	err = p1.TryStart()
	if err != nil {
		log.Errorf("Starting php-cgi(%s) error, because: %v", p1.ExecWithPippedName(), err)
		return err
	}

	go inst.monProcess(p1)

	inst.mutex.Lock()
	if len(inst.processes) < inst.conf.MaxProcesses {
		p1.inPool = true
		inst.processes = append(inst.processes, p1)
	}
	inst.mutex.Unlock()

	p1.TouchIdleTime()
	inst.pushToIdle(p1)

	log.Infof("Started php-cgi(%s), processes: %d, idleProcesses: %d", p1.ExecWithPippedName(), len(inst.processes), len(inst.idleProcesses))

	return err
}

// monProcess 監控 php-cgi 狀態是否跳出
func (inst *Instance) monProcess(p *Process) {
	log.Infof("Starting monitor php-cgi(%s)", p.ExecWithPippedName())
	defer log.Infof("Stopped monitor php-cgi(%s)", p.ExecWithPippedName())

	err := p.cmd.Wait()

	p.closed = true
	inst.remove(p)

	p.inUse = false
	if !inst.stopManage && !inst.isStale(p) {
		inst.createAndPushProcess()
	}

	if log.IsLevelEnabled(log.InfoLevel) {
		log.Debugf("php-cgi(%s) is exited , because : %v", p.execWithPippedName, err)
	}
}

// Stop php-cgi manager , 所有的 process kill
func (inst *Instance) Stop() {
	log.Info("phpfpm stoping.")
	inst.stopManage = true

	for {
		p := inst.popupIdle()
		if p != nil {
			inst.remove(p)
			p.Kill()
		}

		inst.mutex.Lock()
		length := len(inst.processes)
		inst.mutex.Unlock()

		if length == 0 {
			break
		}
		time.Sleep(time.Duration(500 * time.Microsecond))
	}

	log.Info("phpfpm stopped.")
}

// GetIdleProcess : 取得任何一個 Idle 的 Process , 並且移除 Idle 列表
func (inst *Instance) GetIdleProcess() (p *Process) {
	p = inst.popupIdle()

	if p == nil {
		if len(inst.processes) < inst.conf.MaxProcesses {
			inst.createAndPushProcess()
			return inst.GetIdleProcess()
		}
		return
	}

	if p.closed {
		p = nil
		return inst.GetIdleProcess()
	}
	p.inUse = true
	return
}

// PutIdleProcess : 設定 Process 為 idle
func (inst *Instance) PutIdleProcess(p *Process) (err error) {
	if inst.stopManage {
		return
	}

	if p.pipe != nil {
		err = p.pipe.Close()
		p.pipe = nil
	}
	p.TouchIdleTime()

	if inst.isStale(p) {
		log.Warnf("php-cgi(%s) isStale, handled %d requests , need release.", p.execWithPippedName, p.requestCount)
		inst.remove(p)
		p.Kill()
		if p.inPool { // && len(inst.processes) < inst.conf.MaxProcesses
			go inst.createAndPushProcess()
		}
		p = nil
		return
	}

	inst.pushToIdle(p)
	if log.IsLevelEnabled(log.DebugLevel) {
		log.Debugf("php-cgi(%s) is idle , requests count : %d", p.execWithPippedName, p.requestCount)
	}

	return
}

// checkMinIdleConns 检查是否有足够的空闲进程
func (inst *Instance) checkMinIdleProcess() {

	if inst.stopManage {
		return
	}

	if len(inst.processes) >= inst.conf.MinProcesses {
		return
	}

	inst.createAndPushProcess()

}

// release 回收空闲进程
func (inst *Instance) release(d time.Duration) {
	ticker := time.NewTicker(d)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if inst.stopManage {
				return
			}

			log.Infof("time Ticker： php-cgi processes: %d, idleProcesses: %d", len(inst.processes), len(inst.idleProcesses))
			inst.checkMinIdleProcess()
			inst.checkStaleProcess()
		}

	}

}

// checkStaleProcess 检查并回收可回收进程
func (inst *Instance) checkStaleProcess() {
	inst.mutex.Lock()
	if len(inst.idleProcesses) == 0 {
		inst.mutex.Unlock()
		return
	}
	p := inst.idleProcesses[0]
	inst.mutex.Unlock()

	if p == nil {
		log.Infof("inst.idleProcesses[0] is nil")
		return
	}
	if !inst.isStale(p) {
		return
	}

	inst.remove(p)
	p.Kill()

	log.Infof("php-cgi processes %s , isStale, released", p.pippedName)
	return
}

// isStale 是否可回收
func (inst *Instance) isStale(p *Process) bool {
	// return p.requestCount >= inst.conf.MaxRequestsPerProcess ||
	// 	!p.inPool ||
	// 	p.IdleAt().Before(time.Now().Add(-time.Duration(inst.conf.IdleTimeout)))

	defer func() {
		err := recover()
		if err != nil {
			log.Infof("isStale PANIC: %v， %v", p, err)
		}
	}()

	if p.inUse {
		return false
	}
	if !p.inPool || p.closed {
		return true
	}
	if p.requestCount >= inst.conf.MaxRequestsPerProcess {
		return true
	}
	if p.IdleAt().Before(time.Now().Add(-time.Duration(inst.conf.IdleTimeout) * time.Second)) {
		return true
	}
	return false
}

// Remove : 移除
func (inst *Instance) remove(p *Process) {

	inst.mutex.Lock()
	defer inst.mutex.Unlock()

	if len(inst.idleProcesses) > 0 && p == inst.idleProcesses[0] {
		inst.idleProcesses = inst.idleProcesses[1:]
	}

	for i, c := range inst.processes {
		if c == p {
			inst.processes = append(inst.processes[:i], inst.processes[i+1:]...)
			return
		}
	}

}

func (inst *Instance) pushToIdle(p *Process) {
	inst.mutex.Lock()
	inst.idleProcesses = append(inst.idleProcesses, p)
	inst.mutex.Unlock()
}

func (inst *Instance) popupIdle() (p *Process) {
	inst.mutex.Lock()
	if len(inst.idleProcesses) == 0 {
		inst.mutex.Unlock()
		return
	}

	idx := len(inst.idleProcesses) - 1
	p = inst.idleProcesses[idx]
	inst.idleProcesses = inst.idleProcesses[:idx]
	// inst.idleProcessesLen--
	// p.checkMinIdleConns()
	inst.mutex.Unlock()

	return
}
