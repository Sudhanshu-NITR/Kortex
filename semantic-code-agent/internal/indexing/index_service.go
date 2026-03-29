package indexing

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Sudhanshu-NITR/Kortex/semantic-code-agent/internal/domain"
	"github.com/Sudhanshu-NITR/Kortex/semantic-code-agent/internal/embeddings"
	"github.com/Sudhanshu-NITR/Kortex/semantic-code-agent/internal/vectordb"
)

// IndexService orchestrates the ingestion and indexing pipeline.
type IndexService struct {
	loader   *RepoLoader
	chunker  Chunker
	embedder embeddings.Embedder
	store    vectordb.Store
	logger   *slog.Logger
}

func NewIndexService(
	loader *RepoLoader,
	chunker Chunker,
	embedder embeddings.Embedder,
	store vectordb.Store,
	logger *slog.Logger,
) *IndexService {
	return &IndexService{
		loader:   loader,
		chunker:  chunker,
		embedder: embedder,
		store:    store,
		logger:   logger,
	}
}

// IndexRepository runs the full extraction, chunking, embedding, and storage process.
func (s *IndexService) IndexRepository(ctx context.Context, repoPath string) error {
	s.logger.Info("Starting repository indexing", slog.String("path", repoPath))

	// 1. Load source files
	docs, err := s.loader.Load(repoPath)
	if err != nil {
		return fmt.Errorf("failed to load repository: %w", err)
	}

	var allChunks []domain.Chunk

	// 2. Chunk files using AST
	for _, doc := range docs {
		chunks, err := s.chunker.Chunk(ctx, doc)
		if err != nil {
			s.logger.Warn("Failed to chunk document", slog.String("file", doc.FilePath), slog.Any("error", err))
			continue
		}
		allChunks = append(allChunks, chunks...)
	}

	s.logger.Info("Finishing chunking", slog.Int("total_chunks", len(allChunks)))

	// 3. Generate embeddings in batches (e.g., 100 chunks at a time to prevent API )
	batchSize := 10
	for i := 0; i < len(allChunks); i += batchSize {
		end := i + batchSize
		if end > len(allChunks) {
			end = len(allChunks)
		}

		batch := allChunks[i:end]
		var embeddedBatch []domain.Chunk
		var err error

		maxRetries := 5

		for attempt := 0; attempt < maxRetries; attempt++ {
			embeddedBatch, err = s.embedder.EmbedChunks(ctx, batch)
			if err == nil {
				break
			}

			s.logger.Warn("Embedding failed, retrying...",
				slog.Int("attempt", attempt+1),
				slog.Int("start", i),
				slog.Int("end", end),
				slog.Any("error", err),
			)

			// Exponential backoff: 1s, 2s, 4s, 8s...
			time.Sleep(time.Duration(1<<attempt) * time.Second)
		}

		if err != nil {
			s.logger.Error("Failed to embed batch after retries",
				slog.Int("start", i),
				slog.Int("end", end),
				slog.Any("error", err),
			)
			continue
		}

		// Update the main slice with the generated vectors
		for j, chunk := range embeddedBatch {
			allChunks[i+j] = chunk
		}

		time.Sleep(500 * time.Millisecond)
	}

	// 4. Store in Vector Database (only valid embedded chunks)
	var validChunks []domain.Chunk
	for _, chunk := range allChunks {
		if len(chunk.Embedding) > 0 {
			validChunks = append(validChunks, chunk)
		}
	}

	if len(validChunks) > 0 {
		if err := s.store.Upsert(ctx, validChunks); err != nil {
			s.logger.Error("Failed to upsert chunks to vector db", slog.Any("error", err))
			return fmt.Errorf("Failed to upsert chunks: %w", err)
		}
	} else {
		s.logger.Warn("No valid chunks with embeddings to upsert")
	}

	s.logger.Info("Successfully completed repository indexing")
	return nil
}
