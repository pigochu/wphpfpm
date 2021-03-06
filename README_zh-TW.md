# wphpfpm (PHP FastCGI Manager for windows) #

wphpfpm 是我初次練習 Go Lang 開發用來管理 Windows 下的 php-cgi

由於 php-cgi 一次只能服務一個客戶端，除非使用 apache 的 mod_fcgid，不然還真難管理

所以我就自己寫來玩玩，主要是因為用 caddy 來測試 php 只能啟動一隻 php-cgi 實在太不人道了，而且當我修改 php 的設定值，想要重啟 php-cgi，必須要自己手動砍掉 php-cgi 行程

有關於性能，可以參考 [BENCHMARK.md](./BENCHMARK.md)



目前有的功能如下

1. wphpfpm 是獨立的服務，類似 Linux 下的 php-fpm
2. 可以建立不同版本的 php-cgi 來跑
3. php-cgi 可以設定最大啟動的數量
4. 可以安裝於 Windows Service，也可以命令列模式下跑
5. JSON 格式的設定檔

請直接下載 [GO SDK](https://golang.org/) (version 1.12+)後，執行以下命令，就可以得到 wphpfpm.exe

~~~bash
go build
~~~

或者參考 [caddy-wphpfpm-example.md](./caddy-wphpfpm-example.md) 有說明 caddy + wphpfpm 的簡單設定。

## 設定檔說明 ##

以下是 json 範例 , 原始碼中 [config-sample.json](./config-sample.json) 可以下載來修改使用

```json
{
    "LogLevel" : "ERROR",
    "Logger": {
        "FileName": "C:\\wphpfpm\\wphpfpm.log",
        "MaxSize":    10,
        "MaxBackups": 4,
        "MaxAge":     7,
        "Compress":   true,
        "Note": "如果不需要 Log File, 可以將 FileName 設為空字串，則所有 Log 將輸出至 Console , MaxSize 是 MB 為單位 , MaxAge 是以天為單位，本例子為每一份log有7天的內容"
    },
    "Instances" : [
        {
            "Bind" : "127.0.0.1:8000",
            "ExecPath": "C:\\PHP7\\php-cgi.exe",
            "Args" : [],
            "Env": [
                "PHPRC=C:\\PHP7",
                "PHP_FCGI_MAX_REQUESTS=5000" ,
                "PHP_INI_SCAN_DIR=c:\\php7\\conf.d"
            ],
            "MaxProcesses" : 4,
            "MaxRequestsPerProcess": 500
        } ,

        {
            "Bind" : "127.0.0.1:8001",
            "ExecPath": "C:\\PHP5\\php-cgi.exe",
            "Args" : [],
            "Env": [
                "PHPRC=C:\\PHP5",
                "PHP_FCGI_MAX_REQUESTS=5000" ,
                "PHP_INI_SCAN_DIR=c:\\php5\\conf.d"
            ],
            "MaxProcesses" : 2,
            "MaxRequestsPerProcess": 500
        }
    ]
}
```

- LogLevel : 依照等級有以下，預設是 ERROR
  * PANIC
  * FATAL
  * ERROR
  * WARN
  * INFO
  * DEBUG
  * TRACE
  
- Logger : 可以定義 Logger 運作行為

  - FileName : 可以定義 Log 輸出至檔案，如果不需要，可以設定為空字串，輸出會是 Console(stderr)
  - MaxSize : 每一份 Log 檔案最大的 Size , 單位是 MB , 當 Log 檔案已經到達設定值時，會進行 Rotate 的動作。
  - MaxBackups : 最大保留幾份 Log 檔案
  - MaxAge : 每一份檔案保留幾天的內容，單位是天
  - Compress : 是否在 Rotate 之後的檔案要進行壓縮，格式是 gz

- Instances : 定義有多少種 php-cgi 要啟動，這可做為多版本之用

  - Bind : 定義該 instance 要使用甚麼 IP 及 Port ，若針對多版本必須讓不同的 Instances 用不同的 Port 才有效

  - ExecPath : php-cgi 真實路徑

  - Args : 可以帶入 php-cgi 額外參數，**注意，不能使用 -b 的參數**

  - Env : 可以額外加上環境變數

  - MaxProcesses : 最大 php-cgi 執行數量

  - MaxRequestsPerProcess : 每隻 php-cgi 行程，最多能處理幾次請求 , 這個數值必須與 Env 的環境變數 PHP_FCGI_MAX_REQUESTS 一致或小於才不會出問題

- Note : 此欄位並無作用，只是用來註解的



## 使用方式 ##

### 在命令列模式 (Console Mode)下執行 ###

```
wphpfpm run --conf=config.json
```

### 安裝於 Windows Service ###

```
wphpfpm install --conf=c:\wphpfpm\config.json
```

注意，安裝為 Windows Service 模式運作時，必須使用管理者權限才能安裝

### 移除 wphpfpm service ###

```
wphpfpm uninstall
```



### 啟動及停止 wphpfpm service ###

```
wphpfpm start
wphpfpm stop
```

或者，在 Windows 的 **控制台\所有控制台項目\系統管理工具** 下的 **服務** 也可以進行啟動或停止 PHP FastCGI Manager for windows



## wphpfpm 運作的方式

1. wphpfpm 是採用 TCP port 方式對外服務，例如 caddy 當作 Http Server，使用 caddy fastcgi 來連接 wphpfpm 設定值 Instances>Bind 所開啟的 Port
2. wphpfpm 跟 php-cgi 之間的溝通則是採用 windows named pipe 方式溝通，我目前功力仍不夠，不知道如何讓 golang 直接對 php-cgi stdin 溝通，因為看 [xxfpm](https://github.com/78/xxfpm) 的源碼，理論上會更有效率。



## 作者 ##

- Pigo Chu <pigochu@gmail.com>

- Web Site https://www.pigo.idv.tw



## 資源清單 ##

- Windows Service 處理 : https://github.com/chai2010/winsvc
- 命令列處理 : https://gopkg.in/alecthomas/kingpin.v2
- Windows Named Pipe : https://github.com/natefinch/npipe
- 網路文章(https://blog.csdn.net/small_qch/article/details/19562661)
- 處理 Log/LogLevel : Logrus (https://github.com/sirupsen/logrus)
- Log Rotate : lumberjack (https://github.com/natefinch/lumberjack)