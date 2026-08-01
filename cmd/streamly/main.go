package main

import (
	"log/slog"
	"os"

	"streamly/internal/config"
	"streamly/internal/server"
)

func main() {

	cfg, err := config.Load()

	if err != nil {

		slog.Error("config load failed", "err", err)
		os.Exit(1)

	}

	if err := server.Run(cfg); err != nil {

		slog.Error("server stopped", "err", err)
		os.Exit(1)

	}

}
