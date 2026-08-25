package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

func main() {
	rt := &Runtime{
		dataDir:   env("DATA_DIR", "/app/data"),
		publicDir: env("PUBLIC_DIR", "/app/public"),
	}
	if err := rt.loadInstalled(); err != nil {
		fmt.Fprintf(os.Stderr, "startup failed: %v\n", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go rt.startScheduler(ctx)

	r := gin.New()
	r.Use(gin.LoggerWithConfig(gin.LoggerConfig{
		SkipPaths: []string{"/api/system/health"},
	}), gin.Recovery())
	if err := r.SetTrustedProxies(nil); err != nil {
		panic(err)
	}
	r.MaxMultipartMemory = 8 << 20
	r.Static("/uploads", filepath.Join(rt.dataDir, "uploads"))

	api := r.Group("/api")
	rt.registerAPI(api)
	rt.registerStatic(r)

	addr := env("ADDR", ":8080")
	if err := serve(ctx, addr, r); err != nil {
		panic(err)
	}
}

func serve(ctx context.Context, addr string, handler http.Handler) error {
	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return err
		}
		err := <-serverErr
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
