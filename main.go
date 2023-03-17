package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/valyala/fasthttp"
)

var poolGuard chan struct{}

const MaxGoroutine = 512

func init() {
	poolGuard = make(chan struct{}, MaxGoroutine)
}

// func handleTunneling(w http.ResponseWriter, r *http.Request) {
// 	destConn, err := net.DialTimeout("tcp", r.Host, 10*time.Second)
// 	if err != nil {
// 		http.Error(w, err.Error(), http.StatusServiceUnavailable)
// 		return
// 	}
// 	w.WriteHeader(http.StatusOK)
// 	hijacker, ok := w.(http.Hijacker)
// 	if !ok {
// 		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
// 		return
// 	}
// 	clientConn, _, err := hijacker.Hijack()
// 	if err != nil {
// 		http.Error(w, err.Error(), http.StatusServiceUnavailable)
// 	}
// 	go transfer(destConn, clientConn)
// 	go transfer(clientConn, destConn)
// }

func transfer(wg *sync.WaitGroup, destination io.WriteCloser, source io.ReadCloser) {
	defer func() {
		if err := recover(); err != nil {
			log.Print(err)
		}
	}()
	wg.Add(1)
	defer wg.Done()
	// defer destination.Close()
	// defer source.Close()
	if _, err := io.Copy(destination, source); err != nil {
		log.Println(err)
	}
}

func handleHTTP(w http.ResponseWriter, req *http.Request) {
	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()
	copyHeader(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(w, resp.Body); err != nil {
		log.Println(err)
	}
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func acquireRequestPool() {
	poolGuard <- struct{}{} // Would block if guard channel is full
}

func releaseRequestPool() {
	<-poolGuard
}

func handleFastHTTP(ctx *fasthttp.RequestCtx) {

}

func handleFastHTTPS(ctx *fasthttp.RequestCtx) {
	destHost := string(ctx.Request.Header.Peek("Host"))
	destConn, err := fasthttp.DialTimeout(destHost, 10*time.Second)
	if err != nil {
		log.Panic(err)
		return
	}

	ctx.Hijack(func(clientConn net.Conn) {
		defer func() {
			if err := recover(); err != nil {
				log.Print(err)
			}
		}()
		defer clientConn.Close()
		defer destConn.Close()
		wg := sync.WaitGroup{}
		go transfer(&wg, destConn, clientConn)
		go transfer(&wg, clientConn, destConn)
		wg.Wait()
		time.Sleep(3000 * time.Millisecond)
	})

	// fmt.Fprintf(ctx, "Hijacked the connection!")
}

// request handler in fasthttp style, i.e. just plain function.
func fastHTTPHandler(ctx *fasthttp.RequestCtx) {
	switch string(ctx.Method()) {
	case fasthttp.MethodConnect:
		handleFastHTTPS(ctx)
	default:
		handleFastHTTP(ctx)
		fmt.Fprintf(ctx, "Hi there! RequestURI is %q", ctx.RequestURI())
	}
}

func main() {
	// server := &http.Server{
	// 	Addr: ":13002",
	// 	Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	// 		acquireRequestPool()
	// 		defer releaseRequestPool()
	// 		if r.Method == http.MethodConnect {
	// 			handleTunneling(w, r)
	// 		} else {
	// 			handleHTTP(w, r)
	// 		}
	// 	}),
	// 	ReadTimeout:       15 * time.Second,
	// 	ReadHeaderTimeout: 15 * time.Second,
	// 	// Disable HTTP/2.
	// 	TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
	// }

	// pass plain function to fasthttp
	log.Fatal(fasthttp.ListenAndServe(":13002", fastHTTPHandler))
}
