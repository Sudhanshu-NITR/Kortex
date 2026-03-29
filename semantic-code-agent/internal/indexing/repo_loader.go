package indexing

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Sudhanshu-NITR/Kortex/semantic-code-agent/internal/domain"
)

// RepoLoader handles crawling the filesystem to extract source code files.
type RepoLoader struct {
	IgnoredDirs map[string]bool
	IgnoredExts map[string]bool
	logger      *slog.Logger
}

// NewRepoLoader initializes a loader with default rules to strip binaries/large assets.
func NewRepoLoader(logger *slog.Logger) *RepoLoader {
	return &RepoLoader{
		IgnoredDirs: map[string]bool{
			".git": true, "node_modules": true, "vendor": true, "build": true,
		},
		IgnoredExts: map[string]bool{
			".exe": true, ".dll": true, ".png": true, ".jpg": true, ".pdf": true,
			".zip": true, ".tar.gz": true, ".min.js": true, ".json": true, ".ico": true,
		},
		logger: logger,
	}
}

// Load traverses the rootPath and returns a slice of un-chunked Documents.
func (l *RepoLoader) Load(rootPath string) ([]domain.Document, error) {
	var docs []domain.Document
	l.logger.Info("Starting repository load", slog.String("path", rootPath))

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			l.logger.Error("Error accessing path", slog.String("path", path), slog.Any("error", err))
			return err
		}

		if info.IsDir() {
			if l.IgnoredDirs[info.Name()] {
				l.logger.Debug("Skipping directory", slog.String("dir", info.Name()))
				return filepath.SkipDir
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(info.Name()))
		if l.IgnoredExts[ext] {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			l.logger.Warn("Failed to read file", slog.String("path", path), slog.Any("error", err))
			return nil
		}

		docs = append(docs, domain.Document{
			ID:        path,
			FilePath:  path,
			Content:   string(content),
			Language:  strings.TrimPrefix(ext, "."),
			UpdatedAt: time.Now(),
		})

		return nil
	})

	l.logger.Info("Finished repository load", slog.Int("files_loaded", len(docs)))
	return docs, err
}
