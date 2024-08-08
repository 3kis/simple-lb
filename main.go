package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	Retry = iota
	Attempt
)

const (
	MaxRetry   = 3
	MaxAttempt = 3
)

var serverPool ServerPool

type ServerPool struct {
	backends []*Backend
	cur      int32
}

func (s *ServerPool) AppendBackend(b *Backend) {
	s.backends = append(s.backends, b)
}

func (s *ServerPool) GetNextPeer() *Backend {
	nxt := nextIdx()
	l := nxt + len(s.backends)
	for i := nxt; i < l; i++ {
		idx := i % len(s.backends)
		if s.backends[idx].IsAlive() {
			if i != nxt {
				//已经自增过了, 如果挑出来的不是自增后的值, 就需要把最新的值写进去
				//It has been auto-incremented.
				//If the value picked out is not the value after auto-increment,
				//you need to write the latest value into it.
				atomic.StoreInt32(&s.cur, int32(idx))
			}

			return s.backends[idx]
		}
	}
	return nil
}

func (s *ServerPool) HealthCheck() {
	for _, backend := range s.backends {
		status := "up"
		alive := CheckAlive(backend.url)
		if !alive {
			status = "down"
			log.Printf("%s [%s]\n", backend.url, status)
		}
		backend.setAlive(alive)
	}
}

func (s *ServerPool) MarkBackendStatus(backendUrl *url.URL, alive bool) {
	for _, backend := range s.backends {
		if backend.url.String() == backendUrl.String() {
			backend.setAlive(alive)
			return
		}
	}
}

func CheckAlive(u *url.URL) bool {
	conn, err := net.DialTimeout("tcp", u.Host, time.Second*2)

	if err != nil {
		return false
	}
	//If err is not nil, then conn will be empty.
	//If the defer statement is registered in the function before the err judgment statement,
	//it will cause the null pointer to call the Close method and the program will crash.
	//Therefore, you must first judge err and then write defer to register.
	defer conn.Close()
	return true
}

func nextIdx() int {
	return int(atomic.AddInt32(&serverPool.cur, 1) % int32(len(serverPool.backends)))
}

type Backend struct {
	url   *url.URL
	alive bool
	proxy *httputil.ReverseProxy
	mu    sync.RWMutex
}

func (b Backend) IsAlive() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.alive
}

func (b Backend) setAlive(alive bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.alive = alive
}

func main() {
	var serverList string
	var port int
	flag.StringVar(&serverList, "backends", "", "backends to load balance")
	flag.IntVar(&port, "port", 8080, "port to start load balancer")
	flag.Parse()

	if len(serverList) == 0 {
		log.Fatal("no valid backend, please check your input")
	}

	tokens := strings.Split(serverList, ",")
	log.Printf("revice servers : %+v", tokens)
	for _, tok := range tokens {
		backendUrl, err := url.Parse(tok)
		if err != nil {
			log.Fatal("backend url is invalid: ", backendUrl)
		}

		proxy := httputil.NewSingleHostReverseProxy(backendUrl)
		proxy.ErrorHandler = func(writer http.ResponseWriter, request *http.Request, err error) {
			log.Printf("[%s] %s\n", backendUrl.Host, err.Error())
			retries := GetRetryFromContext(request)
			if retries < MaxRetry {
				select {
				//avoid to retry right, we want to wait for a while, so we use time.After
				case <-time.After(time.Millisecond * 10):
					ctx := context.WithValue(request.Context(), Retry, retries+1)
					proxy.ServeHTTP(writer, request.WithContext(ctx))
				}
				return
			}

			log.Printf("site unreachable: ")
			serverPool.MarkBackendStatus(backendUrl, false)
			attempts := GetAttemptFromContext(request)
			ctx := context.WithValue(request.Context(), Attempt, attempts+1)
			log.Printf("Attempt++")
			lb(writer, request.WithContext(ctx))

		}

		serverPool.AppendBackend(&Backend{
			alive: true,
			url:   backendUrl,
			proxy: proxy,
		})

	}
	server := http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: http.HandlerFunc(lb),
	}
	go healthCheck()

	go StartAllServer()

	if err := server.ListenAndServe(); err != nil {
		log.Fatalln("start load balancer fail... please check")
	}
	log.Println(" Load Balancer Start...")
}

func healthCheck() {
	ticker := time.NewTicker(2 * time.Minute)
	for {
		select {
		case <-ticker.C:
			log.Printf("start health check\n")
			serverPool.HealthCheck()
			log.Println("end health check")
		}
	}
}

func lb(writer http.ResponseWriter, request *http.Request) {
	attempts := GetAttemptFromContext(request)
	if attempts > MaxAttempt {
		log.Printf("attempt reach max times!!!\n")
		http.Error(writer, "no available service", http.StatusServiceUnavailable)
		return
	}

	peer := serverPool.GetNextPeer()
	if peer != nil {
		peer.proxy.ServeHTTP(writer, request)
		return
	}

}

func GetAttemptFromContext(request *http.Request) int {
	if attempt, ok := request.Context().Value(Attempt).(int); ok {
		return attempt
	}
	return 0
}

func GetRetryFromContext(request *http.Request) int {
	if retry, ok := request.Context().Value(Retry).(int); ok {
		return retry
	}
	return 0
}

func StartAllServer() {
	backends := []string{"8801", "8802", "8803", "8804"}
	for _, backendPort := range backends {

		go func(port string) {

			h := http.Server{
				Addr: fmt.Sprintf(":%s", port),
				Handler: http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
					writer.Write([]byte(fmt.Sprintf("hello %s", port)))
				}),
			}

			if err := h.ListenAndServe(); err != nil {
				log.Fatalf("failed to start server on port %s: %v", port, err)
			}
			log.Printf("%s start", port)
		}(backendPort)
	}
}
