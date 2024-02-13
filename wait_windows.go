//go:build windows
// +build windows

package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/valyala/fasthttp"
)

func wait(server *fasthttp.Server) <-chan struct{} {
	ctx := make(chan os.Signal)
	sig := make(chan struct{})
	signal.Notify(ctx, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ctx
		server.DisableKeepalive = true
		if err := server.Shutdown(); err != nil {
			Warn("Shutdown err: %s", err)
			defer os.Exit(1)
		} else {
			Info("gracefully stopped")
		}
		sig <- struct{}{}
	}()

	return sig
}
