package phpfpm

import (
	"container/list"
	"sync"
	"wphpfpm/conf"

	log "github.com/sirupsen/logrus"
)

// Instance : struct
type Instance struct {
	execPath  string
	args      []string
	env       []string
	processes []Process
}

var (
	// conf 是 json 讀進來後產生的設定
	phpfpmConf *conf.Conf
	// idleProcesses php-cgi 如果沒有任何連線處理，都存在這
	idleProcesses []*list.List
	stopManage    = false // 如果調用 Stop() , 這個會是 true , 同時 mon() 也不會繼續監控
	mutex         sync.Mutex
)

// Conf : get Json config
func Conf() *conf.Conf {
	return phpfpmConf
}

// Start php-cgi manager
func Start(conf *conf.Conf) (err error) {
	log.Info("phpfpm starting.")
	phpfpmConf = conf
	instanceLen := len(conf.Instances)
	idleProcesses = make([]*list.List, instanceLen)
	for i := 0; i < instanceLen; i++ {
		idleProcesses[i] = list.New()

		for j := 0; j < conf.Instances[i].MaxProcesses; j++ {
			instance := conf.Instances[i]
			p := newProcess(instance.ExecPath, instance.Args, instance.Env)
			p.instanceIndex = i
			err := p.TryStart()
			if err == nil {
				mutex.Lock()
				p.mapElement = idleProcesses[i].PushBack(p)
				mutex.Unlock()
				go monProcess(p)
			} else {
				Stop()
				return err
			}
		}
	}
	log.Info("phpfpm is in loop.")
	return
}

// monProcess 監控 php-cgi 狀態是否跳出
func monProcess(p *Process) {
	log.Infof("Starting monitor php-cgi(%s)", p.ExecWithPippedName())
	defer log.Infof("Stopped monitor php-cgi(%s)", p.ExecWithPippedName())
	for {
		err := p.cmd.Wait()
		if p.requestCount >= phpfpmConf.Instances[p.instanceIndex].MaxRequestsPerProcess {
			err = p.TryStart()
			if err != nil {
				p.restartChan <- false
			} else {
				p.restartChan <- true
			}

			continue
		}
		if err != nil {
			log.Errorf("php-cgi(%s) exit error, because %s", p.ExecWithPippedName(), err.Error())
		}

		mutex.Lock()

		if stopManage {
			// 執行 phpfpm.Stop() 代表不需要再監控了
			mutex.Unlock()
			return
		}

		idleProcesses[p.instanceIndex].Remove(p.mapElement)
		err = p.TryStart()

		if err != nil {
			// 退出監控
			log.Errorf("php-cgi(%s) restart error, because %s", p.ExecWithPippedName(), err.Error())
			mutex.Unlock()
			return
		}
		// 啟動成功
		p.mapElement = idleProcesses[p.instanceIndex].PushBack(p)
		mutex.Unlock()
		if log.IsLevelEnabled(log.InfoLevel) {
			log.Infof("php-cgi(%s) restart successfully.", p.ExecWithPippedName())
		}
	}
}

// Stop php-cgi manager , 所有的 process kill
func Stop() {
	log.Info("phpfpm stoping.")
	stopManage = true
	mutex.Lock()

	var next *list.Element

	for _, v := range idleProcesses {
		for e := v.Front(); e != nil; e = next {
			next = e.Next()
			p := e.Value.(*Process)
			p.Kill()
			v.Remove(e)
		}

	}
	log.Info("phpfpm stopped.")
	mutex.Unlock()
}

// GetIdleProcess : 取得任何一個 Idle 的 Process , 並且移除 Idle 列表
func GetIdleProcess(instanceIndex int) (p *Process) {
	mutex.Lock()
	e := idleProcesses[instanceIndex].Front()
	if e != nil {
		p = idleProcesses[instanceIndex].Remove(e).(*Process)
		p.mapElement = nil
	}
	mutex.Unlock()
	return
}

// PutIdleProcess : 設定 Process 為 idle
func PutIdleProcess(p *Process) (err error) {

	mutex.Lock()

	if p.pipe != nil {
		err = p.pipe.Close()
		p.pipe = nil
	}

	if p.requestCount >= phpfpmConf.Instances[p.instanceIndex].MaxRequestsPerProcess {
		log.Warnf("php-cgi(%s) handled %d requests , need restart.", p.execWithPippedName, p.requestCount)
		p.Kill()
		if true == <-p.restartChan {
			p.mapElement = idleProcesses[p.instanceIndex].PushBack(p)
		} else {
			log.Errorf("php-cgi(%s) restart faild.", p.execWithPippedName)
		}
	} else {
		p.mapElement = idleProcesses[p.instanceIndex].PushBack(p)
		if log.IsLevelEnabled(log.DebugLevel) {
			log.Debugf("php-cgi(%s) is idle , requests count : %d", p.execWithPippedName, p.requestCount)
		}
	}

	mutex.Unlock()
	return
}
