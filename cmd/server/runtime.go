package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2/internal/application"
	"benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2/internal/persistence"
	"benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2/internal/web"
)

type runtime struct {
	listener net.Listener
	server   *http.Server
}

func buildRuntime(address, dataDir string, logger *slog.Logger) (*runtime, error) {
	store, err := persistence.Open(dataDir)
	if err != nil {
		return nil, fmt.Errorf("打开数据存储: %w", err)
	}
	service := application.NewService(store, application.RealClock{})
	handler := web.NewServer(service, logger).Handler()
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("监听 %s: %w", address, err)
	}
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 60 * time.Second}
	return &runtime{listener: listener, server: server}, nil
}

func (r *runtime) serve(errors chan<- error) {
	if err := r.server.Serve(r.listener); err != nil && err != http.ErrServerClosed {
		errors <- err
		return
	}
	errors <- nil
}

func (r *runtime) shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return r.server.Shutdown(ctx)
}

func runServer(cfg config, logger *slog.Logger) error {
	runtime, err := buildRuntime(cfg.address, cfg.dataDir, logger)
	if err != nil {
		return err
	}
	errors := make(chan error, 1)
	go runtime.serve(errors)
	logger.Info("古树迁移保护方案核验台已启动", "address", cfg.address, "data_dir", cfg.dataDir)
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	select {
	case signal := <-signals:
		logger.Info("收到关闭信号", "signal", signal.String())
	case serveErr := <-errors:
		if serveErr != nil {
			return fmt.Errorf("HTTP 服务异常退出: %w", serveErr)
		}
		return nil
	}
	if err := runtime.shutdown(); err != nil {
		return fmt.Errorf("优雅关闭失败: %w", err)
	}
	return <-errors
}
