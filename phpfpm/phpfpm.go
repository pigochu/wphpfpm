package phpfpm

import (
	"container/list"
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"sync"
)

// ConfRoot : JSON root
type ConfRoot struct {
	Instances []ConfInstance
}

// ConfInstance : JSON Instances
type ConfInstance struct {
	Bind         string   `json:"Bind"`
	ExecPath     string   `json:"ExecPath"`
	Args         []string `json:"Args"`
	Env          []string `json:"Env"`
	MaxRequest   int      `json:"MaxRequest"`
	MaxProcesses int      `json:"MaxProcesses"`
}

var (
	// 設定
	conf *ConfRoot

	// 尚未使用的 php-cgi process
	// idleProcesses = make([map[string]*]list.List)
	idleProcesses []*list.List

	// 使用中的 php-cgi process
	// activeProcesses = make(map[string]*list.List)
	activeProcesses []*list.List

	stop = false

	mutex sync.Mutex
)

// InitConfig 初始化 php-cgi manager
// confpath 為 --conf 帶入的設定檔完整路徑及名稱
func InitConfig(confpath string) error {
	var err error
	conf, err = loadJSONFile(confpath)
	log.Println("debug init")
	log.Printf("debug  instances len %d", len(conf.Instances))
	if err == nil {
		// init idle/active processes list

		idleProcesses = make([]*list.List, 2)
		activeProcesses = make([]*list.List, 2)
		for i := 0; i < len(conf.Instances); i++ {
			idleProcesses[i] = new(list.List)
			activeProcesses[i] = new(list.List)

			log.Println("debug create list mapKey " + conf.Instances[i].Bind)
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
				log.Println("Can not start instance " + strconv.Itoa(i) + "php-cgi")
			} else {
				go monProcess(p)
			}
			p.mapElement = idleProcesses[i].PushBack(p)
		}
	}
}

// monProcess 監控 php-cgi 狀態是否跳出
func monProcess(p *Process) {
	for {
		err := p.cmd.Wait()
		mutex.Lock()
		defer mutex.Unlock()
		if stop {
			// 執行 phpfpm.Stop() 代表不需要再監控了
			return
		}

		if err != nil {
			log.Printf("cmd exit , pipe : %s , err : %s\n", p.pippedName, err.Error())
		} else {
			log.Printf("cmd exit , pipe : %s , no err\n", p.pippedName)
		}

		idleProcesses[p.mapIndex].Remove(p.mapElement)
		activeProcesses[p.mapIndex].Remove(p.mapElement)
		err = p.TryStart()

		if err != nil {
			// 啟動成功
			idleProcesses[p.mapIndex].PushBack(p)
			log.Printf("cmd restart , pipe : %s\n", p.pippedName)
		} else {
			// 退出監控

			return
		}
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

	for _, v := range activeProcesses {
		for e := v.Front(); e != nil; e = next {
			next = e.Next()
			p := e.Value.(*Process)
			p.Kill()
			v.Remove(e)
		}
	}
	log.Println("phpfpm stopped")
}

// GetIdleProcess : 取得任何一個 Idle 的 Process
func GetIdleProcess(addrIndex int) *Process {
	mutex.Lock()
	defer mutex.Unlock()
	e := idleProcesses[addrIndex].Front()
	if e != nil {
		p := e.Value.(*Process)
		return p
	}
	return nil
}

// SetProcessActive : 設定 Process active 或 idle
// active = true 代表啟用
// active = false 代表 idle
func SetProcessActive(p *Process, active bool) error {
	mutex.Lock()
	defer mutex.Unlock()

	if active {
		idleProcesses[p.mapIndex].Remove(p.mapElement)
		p.mapElement = activeProcesses[p.mapIndex].PushBack(p)
	} else {
		if p.pipe != nil {
			p.pipe.Close()
			p.pipe = nil
		}
		activeProcesses[p.mapIndex].Remove(p.mapElement)
		p.mapElement = idleProcesses[p.mapIndex].PushBack(p)
	}
	return nil
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
