package indexing

import (
	"context"
	"fmt"
	"log/slog"

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
	batchSize := 100
	for i := 0; i < len(allChunks); i += batchSize {
		end := i + batchSize
		if end > len(allChunks) {
			end = len(allChunks)
		}

		batch := allChunks[i:end]
		embeddedBatch, err := s.embedder.EmbedChunks(ctx, batch)
		if err != nil {
			s.logger.Error("Failed to embed batch", slog.Int("start", i), slog.Int("end", end), slog.Any("error", err))
		}

		// Update the main slice with the generated vectors
		for j, chunk := range embeddedBatch {
			allChunks[i+j] = chunk
		}
	}

	// 4. Store in Vector Database
	if err := s.store.Upsert(ctx, allChunks); err != nil {
		s.logger.Error("Failed to upsert chunks to vector db", slog.Any("error", err))
		return fmt.Errorf("Failed to upsert chunks: %w", err)
	}

	s.logger.Info("Successfully completed repository indexing")
	return nil
}
