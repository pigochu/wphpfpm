# Caddy with wphpfpm setting example #



## Requirement ##

1. [GO SDK](https://golang.org/doc/install)
2. php windows version
3. caddy.exe , build from https://caddyserver.com/
4. wphpfpm source code

## Folder struct example ##

~~~
C:/
    php/				<= decompression php to here 
    caddy/caddy.exe
    caddy/Caddyfile		<= please create it
    caddy/www			<= please create it
    wphpfpm/			<= clone from github 
~~~

### Caddyfile example ###

~~~lua
:2015 {
	root c:\caddy\www
	fastcgi / 127.0.0.1:8000 php
}
~~~

Save it to c:/caddy/Caddyfile , than Run the following  command in console

~~~
c:
cd \caddy
caddy
~~~

caddy will listen on port 2015

## wphpfpm config.json example ##

1. Install GO SDK
2. CD to c:\wphpfpm folder , Run "go build" , you will get a file wphpfpm.exe
3. Copy config-sample.json to config.json
4. Modify config.json as following content

~~~json
{
    "LogLevel" : "DEBUG",
    "Logger": {
        "FileName": "",
        "MaxSize":    1,
        "MaxBackups": 3,
        "MaxAge":     7,
        "Compress":   true
    },
    "Instances" : [
        {
            "Bind" : "127.0.0.1:8000",
            "ExecPath": "C:\\PHP\\php-cgi.exe",
            "Args" : [],
            "Env": [
                "PHPRC=C:\\PHP",
                "PHP_FCGI_MAX_REQUESTS=5000" ,
                "PHP_INI_SCAN_DIR=c:\\php\\conf.d"
            ],
            "MaxProcesses" : 4,
            "MaxRequestsPerProcess": 500
        }
    ]
}
~~~

3. Run the following  command in console

~~~
wphpfpm run --conf=config.json
~~~

If successful, you will see the following output.

~~~
Start in console mode , press CTRL+C to exit ...
.....
2019-09-08 14:43:58 +0800 [info]: Set LogLevel to DEBUG.
2019-09-08 14:43:58 +0800 [info]: Logger ouput set to console.
2019-09-08 14:43:58 +0800 [info]: phpfpm starting.
2019-09-08 14:43:58 +0800 [debug]: Trying to start php-cgi(C:\PHP\php-cgi.exe -> 
2019-09-08 14:43:58 +0800 [info]: Service running ...
2019-09-08 14:43:58 +0800 [debug]: Server 127.0.0.1:8000 starting listener
2019-09-08 14:43:58 +0800 [info]: Server 127.0.0.1:8000 starting accept
2019-09-08 14:43:58 +0800 [debug]: Server 127.0.0.1:8001 starting listener
2019-09-08 14:43:58 +0800 [info]: Server 127.0.0.1:8001 starting accept
~~~

## Test it  ##

Write a test script named phpinfo.php and save to c:\caddy\www

~~~
<?php phpinfo(); ?>
~~~

Open url http://localhost:2015/phpinfo.php

Done.

