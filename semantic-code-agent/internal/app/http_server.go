package app

import (
	"net/http"
	"time"

	"github.com/Sudhanshu-NITR/Kortex/semantic-code-agent/internal/config"
)

func NewHTTPServer(cfg config.HTTPServer, handler http.Handler) *http.Server {

	return &http.Server{
		Addr:         cfg.Addr,
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

}
