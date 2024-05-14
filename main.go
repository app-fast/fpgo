package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"strings"
	"time"
	"unsafe"

	"github.com/appleboy/graceful"
	"github.com/valyala/fasthttp"
	"golang.org/x/exp/slog"
)

const (
	DefaultMaxConcurrent = 512
	DefaultAddr          = ":13002"
	DefaultDNS           = ""
	DefaultTimeout       = 20 * time.Second
)

var (
	addrF          = flag.String("a", DefaultAddr, `Listen address.`)
	maxConcurrentF = flag.Int("c", DefaultMaxConcurrent, "Max concurrency for fasthttp server")
	dnsresolversF  = flag.String("n", "", `DNS nameserves, E.g. "8.8.8.8:53" or "1.1.1.1:53,8.8.8.8:53". Default is empty`)
	timeoutF       = flag.Duration("t", 20*time.Second, `Connection timeout. Examples: 1m or 10s`)
	usageF         = flag.Bool("h", false, "Show usage")

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
		MaxConnWaitTimeout:       3 * time.Second,
	}
)

func Debug(format string, args ...any) {
	slog.Default().Debug(fmt.Sprintf(format, args...))
}

func Info(format string, args ...any) {
	slog.Default().Info(fmt.Sprintf(format, args...))
}

func Warn(format string, args ...any) {
	slog.Default().Warn(fmt.Sprintf(format, args...))
}

func Error(format string, args ...any) {
	slog.Default().Error(fmt.Sprintf(format, args...))
}

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

func randomDNS() string {
	return dns[rand.Intn(len(dns))]
}

func transfer(destination io.WriteCloser, source io.ReadCloser) {
	defer func() {
		if err := recover(); err != nil {
			Warn("transfer: %s", err)
		}
	}()

	if _, err := io.Copy(destination, source); err != nil {
		Debug("transfer io closed: %s", err)
	}
}

func handleFastHTTP(ctx *fasthttp.RequestCtx) {
	if err := fastclient.DoTimeout(&ctx.Request, &ctx.Response, timeout); err != nil {
		Error("Client timeout: %s", err)
	}
}

func handleFastHTTPS(ctx *fasthttp.RequestCtx) {
	if len(ctx.Host()) > 0 {
		Info("Connect to: %s\n", ctx.Host())
	}
	ctx.Hijack(func(clientConn net.Conn) {
		destConn, err := defaultDialer.DialTimeout(b2s(ctx.Host()), 10*time.Second)
		if err != nil {
			Error("Dial timeout: %s", err)
			return
		}

		defer clientConn.Close()
		defer destConn.Close()

		go transfer(destConn, clientConn)
		transfer(clientConn, destConn)
	})
}

// Unsafe but fast []byte to string convertion without memory copy
func b2s(b []byte) string {
	/* #nosec G103 */
	return *(*string)(unsafe.Pointer(&b))
}

// wait graceful shutdown
func wait(server *fasthttp.Server) <-chan struct{} {
	graceful.NewManager().AddRunningJob(func(ctx context.Context) error {
		<-ctx.Done()
		server.DisableKeepalive = true
		if err := server.Shutdown(); err != nil {
			Warn("Shutdown err: %s", err)
			defer os.Exit(1)
		} else {
			Info("gracefully stopped")
		}
		return nil
	})

	return graceful.NewManager().Done()
}

// request handler in fasthttp style, i.e. just plain function.
func fastHTTPHandler(ctx *fasthttp.RequestCtx) {
	switch strings.ToUpper(b2s(ctx.Method())) {
	case fasthttp.MethodConnect:
		handleFastHTTPS(ctx)
	default:
		handleFastHTTP(ctx)
	}
}

func main() {
	server := &fasthttp.Server{
		Handler:            fasthttp.CompressHandler(fastHTTPHandler),
		ReadTimeout:        timeout,
		WriteTimeout:       timeout,
		MaxConnsPerIP:      1000,
		MaxRequestsPerConn: 1000,
		IdleTimeout:        3 * timeout,
		ReduceMemoryUsage:  true,
		CloseOnShutdown:    true,
		Concurrency:        maxConcurrent,
	}

	// Start server
	go func() {
		Info("Concurrency: %d\n", maxConcurrent)
		Info("Nameservers: %s\n", dns)
		Info("Connection timeout is %s\n", timeout)
		Info("listening on address %s\n", addr)
		if err := server.ListenAndServe(addr); err != nil {
			Error("Error in ListenAndServe: %s\n", err)
		}
	}()

	<-wait(server)
}
