# WPHPFPM Benchmark #

- Hardware : Intel E3-1230v2
- RAM : 16GB DDR3
- OS : Windows 10

## Caddy with one php-cgi ##

> Server Software:        Caddy
> Server Hostname:        localhost
> Server Port:            2015
>
> Document Path:          /phpinfo.php
> Document Length:        91015 bytes
>
> Concurrency Level:      10
> Time taken for tests:   6.703 seconds
> Complete requests:      529
> Failed requests:        70
>    (Connect: 0, Receive: 0, Length: 70, Exceptions: 0)
> Non-2xx responses:      32
> Total transferred:      45399235 bytes
> HTML transferred:       45325939 bytes
> Requests per second:    78.92 [#/sec] (mean)
> Time per request:       126.707 [ms] (mean)
> Time per request:       12.671 [ms] (mean, across all concurrent requests)
> Transfer rate:          6614.41 [Kbytes/sec] received



## Caddy with wphpfpm MaxProcess = 4 and LogLevel = DEBUG ##

> Server Software:        Caddy
> Server Hostname:        localhost
> Server Port:            2015
>
> Document Path:          /phpinfo.php
> Document Length:        90796 bytes
>
> Concurrency Level:      10
> Time taken for tests:   5.000 seconds
> Complete requests:      4954
> Failed requests:        491
>    (Connect: 0, Receive: 0, Length: 491, Exceptions: 0)
> Total transferred:      450521646 bytes
> HTML transferred:       449847766 bytes
> Requests per second:    990.80 [#/sec] (mean)
> Time per request:       10.093 [ms] (mean)
> Time per request:       1.009 [ms] (mean, across all concurrent requests)
> Transfer rate:          87992.26 [Kbytes/sec] received