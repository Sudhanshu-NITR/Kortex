package embeddings

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/Sudhanshu-NITR/Kortex/semantic-code-agent/internal/domain"
)

// Embedder defines the interface for generating vector embeddings.
type Embedder interface {
	EmbedChunks(ctx context.Context, chunks []domain.Chunk) ([]domain.Chunk, error)
}

// APIEmbedder implements Embedder by calling an external API.
type APIEmbedder struct {
	Endpoint string
	APIKey   string
	Model    string
	Client   *http.Client
	logger   *slog.Logger
}

func NewAPIEmbedder(endpoint, apiKey, model string, logger *slog.Logger) *APIEmbedder {
	return &APIEmbedder{
		Endpoint: endpoint,
		APIKey:   apiKey,
		Model:    model,
		Client:   &http.Client{},
		logger:   logger,
	}
}

// EmbedChunks calls the API to generate embeddings and populates the Chunk structs.
func (e *APIEmbedder) EmbedChunks(ctx context.Context, chunks []domain.Chunk) ([]domain.Chunk, error) {
	if len(chunks) == 0 {
		return chunks, nil
	}

	e.logger.Info("Generating embeddings", slog.Int("chunk_count", len(chunks)), slog.String("model", e.Model))

	var texts []string
	for _, chunk := range chunks {
		texts = append(texts, chunk.Content)
	}

	// Payload matching standard OpenAI / open-source API formats
	payload := map[string]interface{}{
		"input": texts,
		"model": e.Model,
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", e.Endpoint, bytes.NewBuffer(body))
	if err != nil {
		e.logger.Error("Failed to create request", slog.Any("error", err))
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if e.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+e.APIKey)
	}

	resp, err := e.Client.Do(req)
	if err != nil {
		e.logger.Error("API request failed", slog.Any("error", err))
		return nil, fmt.Errorf("API request failed: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		e.logger.Error("API returned non-OK status", slog.Int("status", resp.StatusCode))
		return nil, fmt.Errorf("API returned status: %d", resp.StatusCode)
	}

	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		e.logger.Error("Failed to decode response", slog.Any("error", err))
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	e.logger.Info("Successfully generated embeddings", slog.Int("chunk_count", len(chunks)))
	for i := range chunks {
		chunks[i].Embedding = result.Data[i].Embedding
	}

	return chunks, nil
}
