package handlers

import (
	"log/slog"
	"net/http"
)

type RootHandler struct {
	log *slog.Logger
}

func NewRootHandler(log *slog.Logger) *RootHandler {
	return &RootHandler{log: log}
}

func (h *RootHandler) Handle(w http.ResponseWriter, r *http.Request) {

	h.log.Info("root endpoint hit")

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Semantic Code Agent running"))
}
