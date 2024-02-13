//go:build !windows
// +build !windows

package main

import (
	"context"
	"os"

	"github.com/appleboy/graceful"
	"github.com/valyala/fasthttp"
)

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
