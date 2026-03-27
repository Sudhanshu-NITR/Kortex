package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/genai"
)

type QueryRequest struct {
	Question string `json:"question"`
}

type QueryResponse struct {
	Answer string `json:"answer"`
	Error  string `json:"error,omitempty"`
}

type QueryHandler struct {
	log       *slog.Logger
	agent     agent.Agent
	runConfig runner.Config
}

// NewQueryHandler safely wraps the ADK Agent inside an HTTP handler
func NewQueryHandler(log *slog.Logger, explainerAgent agent.Agent) (*QueryHandler, error) {
	// Set up the ADK session tracking internally so the LLM remembers previous context
	sessionService := session.InMemoryService()

	runConf := runner.Config{
		AppName:        "semantic-code-agent",
		Agent:          explainerAgent,
		SessionService: sessionService,
	}

	return &QueryHandler{
		log:       log,
		agent:     explainerAgent,
		runConfig: runConf,
	}, nil
}

func (h *QueryHandler) Handle(w http.ResponseWriter, r *http.Request) {
	var req QueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error("Invalid request payload", slog.Any("error", err))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(QueryResponse{Error: "Invalid JSON payload"})
		return
	}

	h.log.Info("Received query request", slog.String("question", req.Question))

	// 1. Initialize an ephemeral runner for this request
	adkRunner, err := runner.New(h.runConfig)
	if err != nil {
		h.log.Error("Failed to initialize ADK runner", slog.Any("error", err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// For a real app, UserID could come from an auth token. We hardcode for now.
	userID := "developer"
	// Create a new session specifically for this run loop to keep history clean
	sessionContext, err := h.runConfig.SessionService.Create(r.Context(), &session.CreateRequest{
		AppName: h.runConfig.AppName,
		UserID:  userID,
	})
	if err != nil {
		h.log.Error("Failed to create session", slog.Any("error", err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// 2. Prepare the GenAI user message
	userMsg := &genai.Content{
		Parts: []*genai.Part{
			genai.NewPartFromText(req.Question),
		},
		Role: string(genai.RoleUser),
	}

	// 3. Execute the Agent Loop Sync and gather the final answer
	var finalAnswer strings.Builder
	h.log.Info("Starting Agent Execution...")

	// runner.Run manages tool-calling automatically until the Agent gives a final response
	eventsEventChannel := adkRunner.Run(
		r.Context(),
		userID,
		sessionContext.Session.ID(),
		userMsg,
		agent.RunConfig{
			StreamingMode: agent.StreamingModeNone, // Turn off streaming for simplicity right now
		},
	)

	for event, runErr := range eventsEventChannel {
		if runErr != nil {
			h.log.Error("Agent execution encountered an error", slog.Any("error", runErr))
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(QueryResponse{Error: "Agent execution failed"})
			return
		}

		if event.Content != nil {
			for _, part := range event.Content.Parts {
				if part.Text != "" {
					finalAnswer.WriteString(part.Text)
				}
			}
		}
	}

	// 4. Return the Answer safely to the HTTP client
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(QueryResponse{Answer: finalAnswer.String()})
	h.log.Info("Successfully served agent response.")
}
