package main

import (
	"io"
	"log"
	"net"
	"sync"
	"time"

	"github.com/valyala/fasthttp"
)

var poolGuard chan struct{}

const MaxGoroutine = 512

func init() {
	poolGuard = make(chan struct{}, MaxGoroutine)
}

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

func acquireRequestPool() {
	poolGuard <- struct{}{} // Would block if guard channel is full
}

func releaseRequestPool() {
	<-poolGuard
}

var client fasthttp.Client

func handleFastHTTP(ctx *fasthttp.RequestCtx) {
	// client := fasthttp.Client{}
	if err := client.DoTimeout(&ctx.Request, &ctx.Response, 10*time.Second); err != nil {
		log.Println(err)
	}
}

func handleFastHTTPS(ctx *fasthttp.RequestCtx) {
	ctx.Hijack(func(clientConn net.Conn) {
		defer func() {
			if err := recover(); err != nil {
				log.Print(err)
			}
		}()

		destHost := string(ctx.Request.Header.Peek("Host"))
		destConn, err := fasthttp.DialTimeout(destHost, 10*time.Second)
		if err != nil {
			log.Println(err)
			return
		}

		defer clientConn.Close()
		defer destConn.Close()
		wg := sync.WaitGroup{}
		go transfer(&wg, destConn, clientConn)
		go transfer(&wg, clientConn, destConn)
		wg.Wait()
		time.Sleep(3000 * time.Millisecond)
	})
}

// request handler in fasthttp style, i.e. just plain function.
func fastHTTPHandler(ctx *fasthttp.RequestCtx) {
	defer func() {
		if err := recover(); err != nil {
			log.Print(err)
		}
	}()
	acquireRequestPool()
	defer releaseRequestPool()
	switch string(ctx.Method()) {
	case fasthttp.MethodConnect:
		handleFastHTTPS(ctx)
	default:
		handleFastHTTP(ctx)
	}
}

func main() {
	defer func() {
		if err := recover(); err != nil {
			log.Print(err)
		}
	}()

	// pass plain function to fasthttp
	log.Fatal(fasthttp.ListenAndServe(":13002", fastHTTPHandler))
}
