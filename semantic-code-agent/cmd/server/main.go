package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Sudhanshu-NITR/Kortex/semantic-code-agent/internal/agents"
	"github.com/Sudhanshu-NITR/Kortex/semantic-code-agent/internal/api"
	"github.com/Sudhanshu-NITR/Kortex/semantic-code-agent/internal/app"
	"github.com/Sudhanshu-NITR/Kortex/semantic-code-agent/internal/config"
	"github.com/Sudhanshu-NITR/Kortex/semantic-code-agent/internal/embeddings"
	"github.com/Sudhanshu-NITR/Kortex/semantic-code-agent/internal/indexing"
	"github.com/Sudhanshu-NITR/Kortex/semantic-code-agent/internal/logger"
	"github.com/Sudhanshu-NITR/Kortex/semantic-code-agent/internal/vectordb"

	"github.com/joho/godotenv"
	"google.golang.org/genai"
)

func main() {
	// 1. Load Configurations & Environment
	cfg := config.MustLoad()
	log := logger.New(cfg.Env)

	if err := godotenv.Load(); err != nil {
		log.Warn("No .env file found; assuming variables are already exported")
	}

	if os.Getenv("GOOGLE_API_KEY") == "" {
		log.Warn("GOOGLE_API_KEY is not set. Components depending on Gemini may fail.")
	}

	// 2. Initialize unified Google GenAI Client
	aiClient, err := genai.NewClient(context.Background(), &genai.ClientConfig{})
	if err != nil {
		log.Error("Failed to initialize Google GenAI client", "error", err)
		os.Exit(1)
	}

	// 3. Initialize the Embedder (Now using Gemini text-embedding-004!)
	embedder := embeddings.NewGeminiEmbedder(
		aiClient,
		"gemini-embedding-001",
		log,
	)

	// 4. Initialize the Vector Store (Qdrant)
	vectorStore, err := vectordb.NewQdrantStore(
		"localhost",
		6334,
		"code_chunks",
		log,
	)
	if err != nil {
		log.Error("Failed to initialize Qdrant store", "error", err)
	} else {
		// Size 768 is exactly what Gemini text-embedding-004 produces!
		if err := vectorStore.Init(context.Background(), 768); err != nil {
			log.Warn("Could not contact Qdrant container, assuming offline for now", "error", err)
		}
	}

	// 5. Initialize Indexing Pipeline (Crawler + AST Chunker)
	repoLoader := indexing.NewRepoLoader(log)
	chunker := indexing.NewASTChunker(log)
	indexService := indexing.NewIndexService(repoLoader, chunker, embedder, vectorStore, log)

	// 6. Initialize the Google ADK Code Explainer Agent
	explainerAgent, err := agents.CreateCodeExplanationAgent(
		context.Background(),
		embedder,
		vectorStore,
		log,
	)
	if err != nil {
		log.Error("Failed to initialize ADK Agent", "error", err)
	} else {
		log.Info("Code Explainer Agent successfully initialized using Gemini!", "agent_name", explainerAgent.Name())
	}

	// 7. Setup Router and Server dependency injection
	router := api.NewRouter(log, explainerAgent, indexService)
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
