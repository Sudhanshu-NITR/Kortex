package indexing

import (
	"context"
	"fmt"
	"log/slog"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/python"

	"github.com/Sudhanshu-NITR/Kortex/semantic-code-agent/internal/domain"
)

// Chunker defines the interface for splitting documents into semantic segments.
type Chunker interface {
	Chunk(ctx context.Context, doc domain.Document) ([]domain.Chunk, error)
}

// ASTChunker uses Tree-sitter to parse code into semantic chunks[cite: 39, 66].
type ASTChunker struct {
	logger *slog.Logger
}

func NewASTChunker(logger *slog.Logger) *ASTChunker {
	return &ASTChunker{logger: logger}
}

func (c *ASTChunker) Chunk(ctx context.Context, doc domain.Document) ([]domain.Chunk, error) {
	c.logger.Debug("Chunking document", slog.String("file", doc.FilePath), slog.String("lang", doc.Language))
	parser := sitter.NewParser()

	switch doc.Language {
	case "go":
		parser.SetLanguage(golang.GetLanguage())
	case "python", "py":
		parser.SetLanguage(python.GetLanguage())
	case "js", "jsx":
		parser.SetLanguage(javascript.GetLanguage())
	default:
		c.logger.Debug("Using fallback chunking", slog.String("file", doc.FilePath))
		return c.fallbackChunking(doc), nil
	}

	tree := parser.Parse(nil, []byte(doc.Content))
	rootNode := tree.RootNode()

	var chunks []domain.Chunk
	c.extractNodes(rootNode, doc, []byte(doc.Content), &chunks)

	c.logger.Debug("Extracted chunks", slog.String("file", doc.FilePath), slog.Int("count", len(chunks)))
	return chunks, nil
}

// extractNodes recursively traverses the AST to find function or method declarations.
func (c *ASTChunker) extractNodes(node *sitter.Node, doc domain.Document, content []byte, chunks *[]domain.Chunk) {
	nodeType := node.Type()

	if nodeType == "function_declaration" || nodeType == "method_declaration" || nodeType == "function_definition" {
		startRow := node.StartPoint().Row
		endRow := node.EndPoint().Row

		chunk := domain.Chunk{
			ID:         fmt.Sprintf("%s-%d", doc.ID, startRow),
			DocumentID: doc.ID,
			Content:    node.Content(content),
			StartLine:  int(startRow),
			EndLine:    int(endRow),
			Metadata: map[string]string{
				"filepath": doc.FilePath,
				"type":     nodeType,
			},
		}
		*chunks = append(*chunks, chunk)
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		c.extractNodes(node.Child(i), doc, content, chunks)
	}
}

func (c *ASTChunker) fallbackChunking(doc domain.Document) []domain.Chunk {
	return []domain.Chunk{{
		ID:         doc.ID + "-fallback",
		DocumentID: doc.ID,
		Content:    doc.Content,
		Metadata:   map[string]string{"filepath": doc.FilePath},
	}}
}
