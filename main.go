package main

import (
	"context"
	"flag"
	"io"
	"log"
	"math/rand"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/appleboy/graceful"
	"github.com/valyala/fasthttp"
)

const (
	DefaultMaxConcurrent = 512
	DefaultAddr          = ":13002"
	DefaultDNS           = "8.8.8.8:53,1.1.1.1:53"
)

var (
	addrF          = flag.String("a", DefaultAddr, `Listen address. Default: ":13002"`)
	maxConcurrentF = flag.Int("c", DefaultMaxConcurrent, "Max concurrency for fasthttp server")
	dnsresolversF  = flag.String("n", "", `DNS nameserves, E.g. "8.8.8.8:53" or "1.1.1.1:53,8.8.8.8:53". Default: ""`)
	timeoutF       = flag.Duration("t", 20*time.Second, `Connection timeout. Examples: 1m or 10s Default: 20s`)
	usageF         = flag.Bool("h", false, "")

	addr          string
	maxConcurrent int
	dns           []string
	timeout       time.Duration

	defaultResolver = &net.Resolver{
		PreferGo:     true,
		StrictErrors: false,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{}
			return d.DialContext(ctx, "udp", randomDNS())
		},
	}

	defaultDialer = fasthttp.TCPDialer{
		Concurrency:      maxConcurrent,
		DNSCacheDuration: time.Minute,
	}

	fastclient = fasthttp.Client{
		NoDefaultUserAgentHeader: true,
		Dial:                     defaultDialer.Dial,
	}
)

func init() {
	flag.Parse()
	if *usageF {
		flag.Usage()
		os.Exit(0)
	}

	addr = *addrF
	maxConcurrent = *maxConcurrentF
	dns = strings.FieldsFunc(*dnsresolversF, func(c rune) bool {
		return c == ','
	})
	timeout = *timeoutF

	if len(dns) > 0 {
		defaultDialer.Resolver = defaultResolver
	}

	rand.Seed(time.Now().UnixNano())
}

// randomGen Generate random int with range [0, max)
func randomGen(max int) int {
	return rand.Intn(max)
}

// randomDNS
func randomDNS() string {
	return dns[randomGen(len(dns))]
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
	if err := fastclient.DoTimeout(&ctx.Request, &ctx.Response, timeout); err != nil {
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
		ReadTimeout:        timeout,
		WriteTimeout:       timeout,
		MaxConnsPerIP:      500,
		MaxRequestsPerConn: 500,
		IdleTimeout:        3 * timeout,
		ReduceMemoryUsage:  true,
		CloseOnShutdown:    true,
		Concurrency:        maxConcurrent,
	}

	// Start server
	go func() {
		log.Printf("Concurrency: %d\n", maxConcurrent)
		log.Printf("Nameservers: %s\n", *dnsresolversF)
		log.Printf("Connection timeout is %s\n", timeout)
		log.Printf("listening on address %s\n", addr)
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
