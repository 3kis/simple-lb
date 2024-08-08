# How to use
```shell
go build -o lb main.go

./lb --backends=http://localhost:8801,http://localhost:8802,http://localhost:8803,http://localhost:8804

```


# Wrk Load Test Res
> wrk -c 40 -d30s http://localhost:8080

>Running 30s test @ http://localhost:8080
2 threads and 40 connections
Thread Stats   Avg      Stdev     Max   +/- Stdev
Latency     8.40ms    9.70ms 103.02ms   86.53%
Req/Sec     3.46k     3.31k   10.24k    72.67%
206748 requests in 30.05s, 25.04MB read
Requests/sec:   6880.72
Transfer/sec:    853.37KB
