package main

import (
	"context"
	"io"
	"log"
	"math/rand"
	"net"
	"os"
	"sync"
	"time"

	"github.com/appleboy/graceful"
	"github.com/valyala/fasthttp"
)

const (
	MaxConcurrent = 512
	addr          = ":13002"
)

var (
	dnsresolvers = []string{"8.8.8.8:53", "1.1.1.1:53"}

	defaultDialer = fasthttp.TCPDialer{
		Concurrency: MaxConcurrent,
		Resolver: &net.Resolver{
			PreferGo:     true,
			StrictErrors: false,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{}
				return d.DialContext(ctx, "udp", randomDNS())
			},
		},
	}

	fastclient = fasthttp.Client{
		NoDefaultUserAgentHeader: true,
		Dial:                     defaultDialer.Dial,
	}
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// randomGen Generate random int with range [0, max)
func randomGen(max int) int {
	return rand.Intn(max)
}

// randomDNS
func randomDNS() string {
	return dnsresolvers[randomGen(len(dnsresolvers))]
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

func handleFastHTTP(ctx *fasthttp.RequestCtx) {
	if err := fastclient.DoTimeout(&ctx.Request, &ctx.Response, 10*time.Second); err != nil {
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

		destConn, err := defaultDialer.DialTimeout(string(ctx.Host()), 10*time.Second)
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
		Handler:            fasthttp.CompressHandler(fastHTTPHandler),
		ReadTimeout:        15 * time.Second,
		WriteTimeout:       15 * time.Second,
		MaxConnsPerIP:      500,
		MaxRequestsPerConn: 500,
		IdleTimeout:        25 * time.Second,
		ReduceMemoryUsage:  true,
		KeepHijackedConns:  true,
		CloseOnShutdown:    true,
		Concurrency:        MaxConcurrent,
	}

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
