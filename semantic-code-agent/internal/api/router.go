package api

import (
	"log/slog"
	"net/http"

	"github.com/Sudhanshu-NITR/Kortex/internal/api/handlers"
)

func NewRouter(log *slog.Logger) *http.ServeMux {

	mux := http.NewServeMux()

	rootHandler := handlers.NewRootHandler(log)

	mux.HandleFunc("GET /", rootHandler.Handle)

	return mux
}
