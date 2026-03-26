package agents

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Sudhanshu-NITR/Kortex/semantic-code-agent/internal/domain"
	"github.com/Sudhanshu-NITR/Kortex/semantic-code-agent/internal/embeddings"
	"github.com/Sudhanshu-NITR/Kortex/semantic-code-agent/internal/vectordb"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
	"google.golang.org/genai"
)

// SearchVectorStoreArgs defines the schema for the tool. ADK uses jsonschema tags.
type SearchVectorStoreArgs struct {
	Query string `json:"query" jsonschema:"The semantic search query string to find relevant code"`
}

// SearchVectorStoreResults defines the output schema for the tool
type SearchVectorStoreResults struct {
	CodeChunks []string `json:"code_chunks"`
	Error      string   `json:"error,omitempty"`
}

// CreateCodeExplanationAgent builds the specialized ADK LlmAgent with our Vector DB tool.
func CreateCodeExplanationAgent(
	ctx context.Context,
	embedder embeddings.Embedder,
	store vectordb.Store,
	logger *slog.Logger,
) (agent.Agent, error) {

	// 1. Define the Custom Tool Function
	// This function handles the RAG embedding and database logic.
	searchToolFunc := func(tctx tool.Context, args SearchVectorStoreArgs) (SearchVectorStoreResults, error) {
		logger.Info("Agent triggered tool: searching vector DB", slog.String("query", args.Query))

		// Embed the query
		queryChunks, err := embedder.EmbedChunks(ctx, []domain.Chunk{{Content: args.Query}})
		if err != nil || len(queryChunks) == 0 {
			return SearchVectorStoreResults{Error: "failed to embed query"}, nil
		}

		// Search the store (getting top 5 chunks)
		results, err := store.Search(ctx, queryChunks[0].Embedding, 5)
		if err != nil {
			return SearchVectorStoreResults{Error: "failed to search vector store"}, nil
		}

		var chunks []string
		for _, r := range results {
			chunks = append(chunks, r.Content)
		}

		return SearchVectorStoreResults{CodeChunks: chunks}, nil
	}

	// 2. Register it as an ADK FunctionTool
	searchTool, err := functiontool.New(
		functiontool.Config{
			Name:        "search_vector_database",
			Description: "Searches the project's vector database to retrieve the semantic code chunks given in a query",
		},
		searchToolFunc,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create the search tool: %w", err)
	}

	// 3. Initialize the Gemini Model using ADK's gemini wrapper
	model, err := gemini.NewModel(ctx, "gemini-2.5-flash", &genai.ClientConfig{})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize model: %w", err)
	}

	// 4. create and return the LlmAgent
	return llmagent.New(llmagent.Config{
		Name:        "code_explainer_agent",
		Model:       model,
		Instruction: "You are an expert Code Explainer. Use the provided tools to search the codebase and answer user questions. Always use the search tool to ground your answers in actual repository code.",
		Description: "An agent that helps users understand their codebase by querying a vector store and summarizing the results.",
		Tools:       []tool.Tool{searchTool},
	})
}
