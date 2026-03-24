package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Sudhanshu-NITR/Kortex/internal/api"
	"github.com/Sudhanshu-NITR/Kortex/internal/app"
	"github.com/Sudhanshu-NITR/Kortex/internal/config"
	"github.com/Sudhanshu-NITR/Kortex/internal/logger"
)

func main() {

	cfg := config.MustLoad()

	log := logger.New(cfg.Env)

	router := api.NewRouter(log)

	server := app.NewHTTPServer(cfg.HTTPServer, router)

	go func() {
		log.Info("starting server", "addr", server.Addr)

		if err := server.ListenAndServe(); err != nil &&
			!errors.Is(err, http.ErrServerClosed) {

			log.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	<-stop
	log.Info("shutdown signal received")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Error("graceful shutdown failed", "error", err)
	}

	log.Info("server stopped")

}
