package vectordb

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Sudhanshu-NITR/Kortex/semantic-code-agent/internal/domain"
	"github.com/qdrant/go-client/qdrant"
)

// Store defines the interface for vector database operations.
type Store interface {
	Upsert(ctx context.Context, chunks []domain.Chunk) error
	Search(ctx context.Context, queryEmbedding []float32, topK int) ([]domain.Chunk, error)
}

// QdrantStore implements Store for Qdrant.
type QdrantStore struct {
	client     *qdrant.Client
	collection string
	logger     *slog.Logger
}

func NewQdrantStore(host string, port int, collection string, logger *slog.Logger) (*QdrantStore, error) {
	client, err := qdrant.NewClient(&qdrant.Config{
		Host: host,
		Port: port,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create qdrant client: %w", err)
	}

	return &QdrantStore{
		client:     client,
		collection: collection,
		logger:     logger,
	}, nil
}

// Init creates a new collection in Qdrant if it doesn't already exist.
// vectorSize specifies the dimensions of the embedding model (e.g. 768 for gemini)
func (s *QdrantStore) Init(ctx context.Context, vectorSize uint64) error {
	s.logger.Info("Initializing vector schema in Qdrant")

	exists, err := s.client.CollectionExists(ctx, s.collection)
	if err != nil {
		return fmt.Errorf("failed to check qdrant collection: %w", err)
	}

	if !exists {
		s.logger.Info("Creating Qdrant collection", slog.String("name", s.collection))
		err = s.client.CreateCollection(ctx, &qdrant.CreateCollection{
			CollectionName: s.collection,
			VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
				Size:     vectorSize,
				Distance: qdrant.Distance_Cosine, // Using Cosine similarity perfectly maps to Sentence-Transformers/Gemini outputs
			}),
		})
		if err != nil {
			return fmt.Errorf("failed to create qdrant collection: %w", err)
		}
	}
	return nil
}

func (s *QdrantStore) Upsert(ctx context.Context, chunks []domain.Chunk) error {
	if len(chunks) == 0 {
		return nil
	}

	s.logger.Info("Upserting chunks into Qdrant", slog.Int("count", len(chunks)))

	var points []*qdrant.PointStruct
	for _, chunk := range chunks {
		// Map the fields from your core Chunk into the Qdrant Payload for retrieval
		payload := map[string]any{
			"document_id": chunk.DocumentID,
			"content":     chunk.Content,
			"start_line":  int64(chunk.StartLine),
			"end_line":    int64(chunk.EndLine),
		}

		// Include any extra metadata
		for k, v := range chunk.Metadata {
			payload[k] = v
		}

		point := &qdrant.PointStruct{
			// Note: Qdrant requires IDs to be UUIDs. Chunk.ID must be a valid RFC4122 string
			// generated during ingestion.
			Id:      qdrant.NewIDUUID(chunk.ID),
			Vectors: qdrant.NewVectors(chunk.Embedding...),
			Payload: qdrant.NewValueMap(payload),
		}
		points = append(points, point)
	}

	wait := true
	_, err := s.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: s.collection,
		Wait:           &wait,
		Points:         points,
	})

	if err != nil {
		s.logger.Error("Failed to upsert points", slog.Any("error", err))
		return fmt.Errorf("failed to upsert chunks into qdrant: %w", err)
	}

	s.logger.Info("Successfully returned batch to Qdrant")
	return nil
}

func (s *QdrantStore) Search(ctx context.Context, queryEmbedding []float32, topK int) ([]domain.Chunk, error) {
	s.logger.Info("Searching Qdrant collection", slog.Int("topK", topK))

	limit := uint64(topK)
	searchResults, err := s.client.Query(ctx, &qdrant.QueryPoints{
		CollectionName: s.collection,
		Query:          qdrant.NewQuery(queryEmbedding...),
		Limit:          &limit,
		WithPayload:    qdrant.NewWithPayload(true),
	})

	if err != nil {
		return nil, fmt.Errorf("qdrant search failed: %w", err)
	}

	var results []domain.Chunk
	for _, scoredPoint := range searchResults {
		chunk := domain.Chunk{
			ID:       scoredPoint.Id.GetUuid(),
			Metadata: make(map[string]string),
		}

		// Reconstruct core chunk fields from the stored Qdrant payload
		payload := scoredPoint.GetPayload()
		if p, ok := payload["document_id"]; ok {
			chunk.DocumentID = p.GetStringValue()
		}
		if p, ok := payload["content"]; ok {
			chunk.Content = p.GetStringValue()
		}
		if p, ok := payload["start_line"]; ok {
			chunk.StartLine = int(p.GetIntegerValue())
		}
		if p, ok := payload["end_line"]; ok {
			chunk.EndLine = int(p.GetIntegerValue())
		}

		results = append(results, chunk)
	}

	return results, nil
}
