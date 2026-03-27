package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/Sudhanshu-NITR/Kortex/semantic-code-agent/internal/indexing"
)

type IndexRequest struct {
	RepoPath string `json:"repo_path"`
}

type IndexResponse struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

type IndexHandler struct {
	log          *slog.Logger
	indexService *indexing.IndexService
}

// NewIndexHandler builds the handler wrapping our underlying Index Service
func NewIndexHandler(log *slog.Logger, indexService *indexing.IndexService) *IndexHandler {
	return &IndexHandler{
		log:          log,
		indexService: indexService,
	}
}

func (h *IndexHandler) Handle(w http.ResponseWriter, r *http.Request) {
	var req IndexRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error("Invalid request payload", slog.Any("error", err))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(IndexResponse{Error: "Invalid JSON payload"})
		return
	}

	if req.RepoPath == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(IndexResponse{Error: "repo_path is required"})
		return
	}

	h.log.Info("Starting repository indexing via API", slog.String("path", req.RepoPath))

	// Trigger the ingestion pipeline!
	err := h.indexService.IndexRepository(r.Context(), req.RepoPath)
	if err != nil {
		h.log.Error("Indexing failed", slog.Any("error", err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(IndexResponse{Error: "Failed to index the repository: " + err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(IndexResponse{Status: "Successfully indexed repository to vector DB"})
	h.log.Info("Successfully served indexing response.")
}
