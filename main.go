package main

import (
	"context"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"github.com/appleboy/graceful"
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
	defer wg.Done()
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
		wg.Add(2)
		go transfer(&wg, destConn, clientConn)
		go transfer(&wg, clientConn, destConn)
		wg.Wait()
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

	server := &fasthttp.Server{
		Handler:              fastHTTPHandler,
		ReadTimeout:          15 * time.Second,
		WriteTimeout:         15 * time.Second,
		MaxConnsPerIP:        500,
		MaxRequestsPerConn:   500,
		MaxKeepaliveDuration: 25 * time.Second,
	}

	addr := ":13002"
	// Start server
	go func() {
		log.Printf("%s listening on address %s\n", server.Name, addr)
		if err := server.ListenAndServe(addr); err != nil {
			log.Fatalf("Error in ListenAndServe: %s\n", err)
		}
	}()

	graceful.NewManager().AddRunningJob(func(ctx context.Context) error {
		<-ctx.Done()
		server.DisableKeepalive = true
		if err := server.Shutdown(); err != nil {
			log.Println("Shutdown err", err)
			defer os.Exit(1)
		} else {
			log.Println("gracefully stopped")
		}
		return nil
	})

	<-graceful.NewManager().Done()
}
