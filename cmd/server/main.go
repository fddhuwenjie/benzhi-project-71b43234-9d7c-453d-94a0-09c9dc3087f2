package main

import (
	"log/slog"
	"os"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	cfg, err := parseConfig(os.Args[1:])
	if err != nil {
		logger.Error("配置无效", "error", err)
		os.Exit(2)
	}
	if cfg.selftest {
		err = runSelftest(cfg, logger)
	} else {
		err = runServer(cfg, logger)
	}
	if err != nil {
		logger.Error("服务退出", "error", err)
		os.Exit(1)
	}
}
