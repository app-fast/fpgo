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
)

type LogLevel uint8

const (
	DefaultMaxConcurrent = 512
	DefaultAddr          = ":13002"
	DefaultDNS           = ""
	DefaultTimeout       = 60 * time.Second
	DefaultLogLevel      = 1

	LogLevelDebug LogLevel = 0
	LogLevelInfo  LogLevel = 1
	LogLevelWarn  LogLevel = 2
	LogLevelError LogLevel = 3
)

var (
	version = "dev"

	addrF          = flag.String("a", DefaultAddr, `Listen address.`)
	maxConcurrentF = flag.Int("c", DefaultMaxConcurrent, "Max concurrency for fasthttp server")
	dnsresolversF  = flag.String("n", DefaultDNS, `DNS nameserves, E.g. "8.8.8.8:53" or "1.1.1.1:53,8.8.8.8:53". Default is empty`)
	timeoutF       = flag.Duration("t", DefaultTimeout, `Connection timeout. Examples: 1m or 10s`)
	logLevelF      = flag.Int("l", DefaultLogLevel, `Log level. Examples: 0 (debug), 1 (info), 2 (warn), 3 (error). Default is 1`)
	usageF         = flag.Bool("h", false, "Show usage")
	verF           = flag.Bool("v", false, "Show version")

	addr          string
	maxConcurrent int
	dns           []string
	timeout       time.Duration
	logLevel      LogLevel
	ver           string

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
		Dial:                     defaultDialer.DialDualStack,
		MaxConnWaitTimeout:       10 * time.Second,
	}
)

func Debug(format string, args ...any) {
	if logLevel > 0 {
		return
	}
	if len(args) == 0 {
		format += "\n" // append line break if no arg
	}
	fmt.Printf("%s %s %s",
		time.Now().Local().Format("[2006-01-02 15:04:05]"),
		"DEBUG",
		fmt.Sprintf(format, args...))
}

func Info(format string, args ...any) {
	if logLevel > 1 {
		return
	}
	if len(args) == 0 {
		format += "\n" // append line break if no arg
	}
	fmt.Printf("%s %s %s",
		time.Now().Local().Format("[2006-01-02 15:04:05]"),
		"INFO",
		fmt.Sprintf(format, args...))
}

func Warn(format string, args ...any) {
	if logLevel > 2 {
		return
	}
	if len(args) == 0 {
		format += "\n" // append line break if no arg
	}
	fmt.Printf("%s %s %s",
		time.Now().Local().Format("[2006-01-02 15:04:05]"),
		"WARN",
		fmt.Sprintf(format, args...))
}

func Error(format string, args ...any) {
	if logLevel > 3 {
		return
	}
	if len(args) == 0 {
		format += "\n" // append line break if no arg
	}
	fmt.Printf("%s %s %s",
		time.Now().Local().Format("[2006-01-02 15:04:05]"),
		"ERROR",
		fmt.Sprintf(format, args...))
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

	logLevel = LogLevel(*logLevelF)

	if *verF {
		println(version)
		os.Exit(0)
	}

	ver = version
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
	Info("Connect to: http://%s\n", ctx.Host())
	if err := fastclient.DoTimeout(&ctx.Request, &ctx.Response, timeout); err != nil {
		Error("Client timeout: %s", err)
	}
}

func handleFastHTTPS(ctx *fasthttp.RequestCtx) {
	Info("Connect to: https://%s\n", ctx.Host())
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
		Info("Version: %s\n", ver)
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
