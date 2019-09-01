package phpfpm

import (
	"container/list"
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"sync"
)

// ConfRoot : JSON root
type ConfRoot struct {
	Instances []ConfInstance
}

// ConfInstance : JSON Instances
type ConfInstance struct {
	Bind     string   `json:"Bind"`
	ExecPath string   `json:"ExecPath"`
	Args     []string `json:"Args"`
	Env      []string `json:"Env"`
	// MaxRequestsPerProcess 每個php-cgi行程最多能夠處理幾次要求
	MaxRequestsPerProcess int `json:"MaxRequestsPerProcess"`
	MaxProcesses          int `json:"MaxProcesses"`
}

var (
	// 設定
	conf *ConfRoot

	// 尚未使用的 php-cgi process
	// idleProcesses = make([map[string]*]list.List)
	idleProcesses []*list.List

	// 使用中的 php-cgi process
	// activeProcesses = make(map[string]*list.List)
	// activeProcesses []*list.List

	stop  = false
	mutex sync.Mutex
)

// InitConfig 初始化 php-cgi manager
// confpath 為 --conf 帶入的設定檔完整路徑及名稱
func InitConfig(confpath string) error {
	var err error
	conf, err = loadJSONFile(confpath)
	if err == nil {
		// init idle/active processes list
		instanceLen := len(conf.Instances)
		idleProcesses = make([]*list.List, instanceLen)

		for i := 0; i < instanceLen; i++ {
			idleProcesses[i] = new(list.List)
		}
	}

	return err
}

// Conf : get Json config
func Conf() *ConfRoot {
	return conf
}

// Start php-cgi manager
func Start() {
	for i := 0; i < len(conf.Instances); i++ {
		idleProcesses[i] = list.New()

		for j := 0; j < conf.Instances[i].MaxProcesses; j++ {
			instance := conf.Instances[i]
			p := newProcess(instance.ExecPath, instance.Args, instance.Env)
			p.mapIndex = i
			err := p.TryStart()
			if err != nil {
				log.Printf("Can not start instance %d\n", i)
			} else {
				mutex.Lock()
				p.mapElement = idleProcesses[i].PushBack(p)
				mutex.Unlock()
				go monProcess(p)
			}
		}
	}
}

// monProcess 監控 php-cgi 狀態是否跳出
func monProcess(p *Process) {
	for {

		log.Printf("Mon php-cgi %s\n", p.pippedName)

		err := p.cmd.Wait()
		if p.requestCount >= conf.Instances[p.mapIndex].MaxRequestsPerProcess {
			err = p.TryStart()
			if err != nil {
				p.restartChan <- false
			} else {
				p.restartChan <- true
			}

			continue
		}

		mutex.Lock()

		if stop {
			// 執行 phpfpm.Stop() 代表不需要再監控了
			mutex.Unlock()
			return
		}

		if err != nil {
			log.Printf("php-cgi exit , pipe : %s , err : %s\n", p.pippedName, err.Error())
		} else {
			log.Printf("php-cgi exit , pipe : %s , no err\n", p.pippedName)
		}

		idleProcesses[p.mapIndex].Remove(p.mapElement)
		err = p.TryStart()

		if err != nil {
			// 退出監控
			log.Printf("php-cgi restart error : %s , pipe : %s\n", err.Error(), p.pippedName)
			mutex.Unlock()
			return
		}
		// 啟動成功
		p.mapElement = idleProcesses[p.mapIndex].PushBack(p)
		log.Printf("php-cgi restart , pipe : %s\n", p.pippedName)
		mutex.Unlock()
	}
}

// Stop php-cgi manager , 所有的 process kill
func Stop() {
	log.Println("phpfpm stoping")
	stop = true
	mutex.Lock()
	defer mutex.Unlock()

	var next *list.Element

	for _, v := range idleProcesses {
		for e := v.Front(); e != nil; e = next {
			next = e.Next()
			p := e.Value.(*Process)
			p.Kill()
			v.Remove(e)
		}

	}

	log.Println("phpfpm stopped")
}

// GetIdleProcess : 取得任何一個 Idle 的 Process , 並且 Active
func GetIdleProcess(addrIndex int) *Process {
	mutex.Lock()
	defer mutex.Unlock()
	e := idleProcesses[addrIndex].Front()
	if e != nil {
		p := e.Value.(*Process)
		idleProcesses[p.mapIndex].Remove(p.mapElement)
		return p
	}
	return nil
}

// PutIdleProcess : 設定 Process 為 idle
func PutIdleProcess(p *Process) (err error) {

	mutex.Lock()
	defer mutex.Unlock()
	if p.pipe != nil {
		err = p.pipe.Close()
		p.pipe = nil
	}

	if p.requestCount >= conf.Instances[p.mapIndex].MaxRequestsPerProcess {
		if true == <-p.restartChan {
			p.mapElement = idleProcesses[p.mapIndex].PushBack(p)
		}
	} else {
		p.mapElement = idleProcesses[p.mapIndex].PushBack(p)
	}
	return
}

// loadJSONFile 讀取 JSON 設定檔，並返回 *Config
func loadJSONFile(filePath string) (*ConfRoot, error) {
	jsonFile, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer jsonFile.Close()

	conf := new(ConfRoot)
	byteValue, _ := ioutil.ReadAll(jsonFile)
	json.Unmarshal(byteValue, &conf)
	return conf, nil
}
