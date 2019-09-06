# wphpfpm (PHP FastCGI Manager for windows) #

wphpfpm is my first go-lang project for manage php-cgi on Windows.

Since php-cgi can only serve one client at one time, unless you use apache's mod_fcgid, it's really hard to manage.

So I wrote it for myself, mainly because using caddy to test php can only start one php-cgi process. It is too inhuman, and when I want to change the php settings and want to restart php-cgi, I have to manually kill php-cgi.

For performance, please refer to  [BENCHMARK.md](./BENCHMARK.md)



## Features ##

1. wphpfpm is a standalone service, similar to php-fpm under Linux
2. You can create multiple instances for multiple version php-cgi
3. php-cgi can set the maximum number of process
4. wphpfpm can be a windows service or running on console mode.
5. JSON format configuration file

Please download GO  [GO SDK](https://golang.org/)  (version 1.12+) and execute the following command to get wphpfpm.exe

~~~bash
go build
~~~



## Configure file  ##

The following is a json example. The source code [config-sample.json](./config-sample.json) can be download and modify it for your environment.

```json
{
    "LogLevel" : "ERROR",
    "Logger": {
        "FileName": "C:\\wphpfpm\\wphpfpm.log",
        "MaxSize":    10,
        "MaxBackups": 4,
        "MaxAge":     7,
        "Compress":   true,
        "Note": "If you don't need Logger, you can remove the entire Logger section, MaxSize is the unit of MB, MaxAge is the unit of days, this example has 7 days of content per log."
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

- LogLevel : According to the level, the default is ERROR
  * PANIC
  * FATAL
  * ERROR
  * WARN
  * INFO
  * DEBUG
  * TRACE
- Logger : You can define the Log output to the file. If you don't need it, you can remove it. The output will be Console (stderr).
- Instances : Define how many kinds of php-cgi to start, this can be used as multiple versions
- Bind : Define what IP and Port to use for this instance. If multiple versions are required, different Instances must be used with different Ports.
- ExecPath : php-cgi real  path.
- Args : You can add parameters for execute php-cgi.exe, note that you can't use  -b  parameters
- Env : Additional environmental variables
- MaxProcesses : This directive sets the maximum number of php-cgi processes which can be active at one time.
- MaxRequestsPerProcess : Each php-cgi  process trip can handle up to several requests. This value must be the same or less than Env's environment variable PHP_FCGI_MAX_REQUESTS.
- Note : This field has no effect, just for comment



## Usage ##

### Run in command line mode (console mode) ###

```
wphpfpm run --conf=config.json
```

### Install as Windows Service ###

```
wphpfpm install --conf=c:\wphpfpm\config.json
```

Note that when install as  a Windows Service, you must use administrator privileges to install.

### Remove wphpfpm service ###

```
wphpfpm uninstall
```



### Start and stop wphpfpm service ###

```
wphpfpm start
wphpfpm stop
```

Alternatively, the service under Windows Control Panel\All Control Panel Items\Administrative Tools\Services can also be started or stopped PHP FastCGI Manager for windows



## Resouces ##

- Windows Service Control : https://github.com/chai2010/winsvc
- Command Line parser  : https://gopkg.in/alecthomas/kingpin.v2
- Windows Named Pipe : https://github.com/natefinch/npipe
- Article (https://blog.csdn.net/small_qch/article/details/19562661)
- Log/LogLevel : Logrus (https://github.com/sirupsen/logrus)
- Log Rotate : lumberjack (https://github.com/natefinch/lumberjack)