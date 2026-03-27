package api

import (
	"log/slog"
	"net/http"

	"github.com/Sudhanshu-NITR/Kortex/semantic-code-agent/internal/api/handlers"
	"github.com/Sudhanshu-NITR/Kortex/semantic-code-agent/internal/indexing"
	"google.golang.org/adk/agent"
)

func NewRouter(log *slog.Logger, explainerAgent agent.Agent, indexService *indexing.IndexService) *http.ServeMux {
	mux := http.NewServeMux()

	// Generic root
	rootHandler := handlers.NewRootHandler(log)
	mux.HandleFunc("GET /", rootHandler.Handle)

	// Register QA endpoint
	if explainerAgent != nil {
		queryHandler, err := handlers.NewQueryHandler(log, explainerAgent)
		if err == nil {
			mux.HandleFunc("POST /api/query", queryHandler.Handle)
		} else {
			log.Error("Failed to initialize query handler", "error", err)
		}
	}

	// Register index endpoint
	if indexService != nil {
		indexHandler := handlers.NewIndexHandler(log, indexService)
		mux.HandleFunc("POST /api/index", indexHandler.Handle)
	}

	return mux
}
