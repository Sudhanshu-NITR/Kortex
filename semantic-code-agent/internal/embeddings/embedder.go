package embeddings

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Sudhanshu-NITR/Kortex/semantic-code-agent/internal/domain"
	"google.golang.org/genai"
)

// Embedder defines the interface for generating vector embeddings.
type Embedder interface {
	EmbedChunks(ctx context.Context, chunks []domain.Chunk) ([]domain.Chunk, error)
	EmbedQuery(ctx context.Context, query string) ([]float32, error)
}

// GeminiEmbedder implements Embedder using the official Google GenAI Go SDK.
type GeminiEmbedder struct {
	client *genai.Client
	model  string
	logger *slog.Logger
}

func NewGeminiEmbedder(client *genai.Client, model string, logger *slog.Logger) *GeminiEmbedder {
	return &GeminiEmbedder{
		client: client,
		model:  model,
		logger: logger,
	}
}

// EmbedChunks calls the Gemini API to generate embeddings and populates the Chunk structs.
func (e *GeminiEmbedder) EmbedChunks(ctx context.Context, chunks []domain.Chunk) ([]domain.Chunk, error) {
	if len(chunks) == 0 {
		return chunks, nil
	}

	e.logger.Info("Generating embeddings with Gemini", slog.Int("chunk_count", len(chunks)), slog.String("model", e.model))

	var contents []*genai.Content
	for _, chunk := range chunks {
		// Prepare the content structure expected by GenAI EmbedContent
		contents = append(contents, &genai.Content{
			Parts: []*genai.Part{
				genai.NewPartFromText(chunk.Content),
			},
		})
	}

	// Make the API request to Gemini
	dim := int32(768)
	config := &genai.EmbedContentConfig{
		TaskType:             "RETRIEVAL_DOCUMENT",
		OutputDimensionality: &dim,
	}
	resp, err := e.client.Models.EmbedContent(ctx, e.model, contents, config)
	if err != nil {
		e.logger.Error("Gemini EmbedContent API request failed", slog.Any("error", err))
		return nil, fmt.Errorf("gemini api request failed: %w", err)
	}

	if len(resp.Embeddings) != len(chunks) {
		return nil, fmt.Errorf("embedding mismatch: expected %d embeddings, got %d", len(chunks), len(resp.Embeddings))
	}

	e.logger.Info("Successfully generated Gemini embeddings", slog.Int("chunk_count", len(chunks)))
	for i := range chunks {
		chunks[i].Embedding = resp.Embeddings[i].Values
	}

	return chunks, nil
}

// EmbedQuery calls the Gemini API to generate an embedding optimized for retrieval query.
func (e *GeminiEmbedder) EmbedQuery(ctx context.Context, query string) ([]float32, error) {
	e.logger.Info("Generating query embedding with Gemini", slog.String("model", e.model))

	contents := []*genai.Content{{
		Parts: []*genai.Part{
			genai.NewPartFromText(query),
		},
	}}

	dim := int32(768)
	config := &genai.EmbedContentConfig{
		TaskType:             "RETRIEVAL_QUERY",
		OutputDimensionality: &dim,
	}
	resp, err := e.client.Models.EmbedContent(ctx, e.model, contents, config)
	if err != nil {
		e.logger.Error("Gemini EmbedContent API request failed for query", slog.Any("error", err))
		return nil, fmt.Errorf("gemini api request failed for query: %w", err)
	}
	if len(resp.Embeddings) == 0 {
		return nil, fmt.Errorf("no embedding returned for query")
	}

	return resp.Embeddings[0].Values, nil
}
