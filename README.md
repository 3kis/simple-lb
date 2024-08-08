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
Latency     7.69ms    9.67ms  93.71ms   86.84%
Req/Sec     4.24k     3.92k   12.01k    69.50%
253368 requests in 30.05s, 30.69MB read
Requests/sec:   8431.81
Transfer/sec:      1.02MB
