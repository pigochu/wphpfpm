package conf

import (
	"encoding/json"
	"io/ioutil"
	"os"
)

// Conf : JSON root
type Conf struct {
	// Instances 為陣列，包含了多個 ConfInstance
	Instances []Instance
	LogLevel  string  `json:"LogLevel"`
	Logger    *Logger `json:"Logger"`
}

// Instance : JSON Instances
type Instance struct {
	Bind     string   `json:"Bind"`
	ExecPath string   `json:"ExecPath"`
	Args     []string `json:"Args"`
	Env      []string `json:"Env"`
	// MaxRequestsPerProcess 每個php-cgi行程最多能夠處理幾次要求 , Default 5000
	MaxRequestsPerProcess int `json:"MaxRequestsPerProcess,5000"`
	// MaxProcesses 定義 Instance 啟動 php-cgi 的最大數量，default 4
	MaxProcesses int `json:"MaxProcesses,4"`
	// MaxProcesses 定義 Instance 啟動 php-cgi 的最大數量，default 2
	MinProcesses int `json:"MinProcesses,2"`
	// IdleTimeout 回收空闲时间超过 IdleTimeout 秒的进程，default 20s, set 0 to disable
	IdleTimeout int `json:"IdleTimeout,20"`
	// Note 只是註解，此欄位沒有任何作用
	Note string `json:"-"`
}

// Logger : the same lumberjack.Logger
// see https://github.com/natefinch/lumberjack
type Logger struct {
	Filename   string `json:"Filename"`
	MaxSize    int    `json:"MaxSize,10"`
	MaxAge     int    `json:"MaxAge,7"`
	MaxBackups int    `json:"MaxBackups,4"`
	LocalTime  bool   `json:"LocalTime,true"`
	Compress   bool   `json:"Compress,false"`
}

// LoadFile 讀取 JSON 設定檔，並返回 *Conf
func LoadFile(filePath string) (conf *Conf, err error) {

	jsonFile, err := os.Open(filePath)
	if err != nil {
		return
	}
	defer jsonFile.Close()

	conf = &Conf{LogLevel: "ERROR"}
	byteValue, _ := ioutil.ReadAll(jsonFile)
	err = json.Unmarshal(byteValue, &conf)
	return
}

// FileExist config file is exist 檢查檔案是否存在
func FileExist(filePath string) bool {
	if _, err := os.Stat(filePath); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}
