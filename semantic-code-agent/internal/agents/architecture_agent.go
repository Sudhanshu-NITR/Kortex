package agents

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
	"google.golang.org/genai"
)

// ProjectStructureArgs defines the input for the directory mapping tool.
type ProjectStructureArgs struct {
	RootPath string `json:"root_path" jsonschema:"The absolute path to the project root to map"`
}

// ProjectStructureResults defines the output for the directory mapping tool.
type ProjectStructureResults struct {
	Tree  string `json:"tree"`
	Error string `json:"error,omitempty"`
}

// CreateArchitectureAgent builds an agent focused on high-level codebase structure.
func CreateArchitectureAgent(
	ctx context.Context,
	logger *slog.Logger,
) (agent.Agent, error) {

	// 1. Define the "Map Directory" Tool
	// This tool allows the agent to see the folder hierarchy without reading every file.
	mapDirFunc := func(tctx tool.Context, args ProjectStructureArgs) (ProjectStructureResults, error) {
		logger.Info("Agent triggered tool: mapping directory structure", slog.String("path", args.RootPath))

		var tree strings.Builder
		err := filepath.Walk(args.RootPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			// Ignore hidden files and specific large folders
			if info.IsDir() && (strings.HasPrefix(info.Name(), ".") || info.Name() == "node_modules" || info.Name() == "vendor") {
				return filepath.SkipDir
			}

			rel, _ := filepath.Rel(args.RootPath, path)
			if rel == "." {
				return nil
			}

			depth := strings.Count(rel, string(os.PathSeparator))
			// Only show up to 5 levels deep for a "high-level" overview
			if depth > 4 {
				return nil
			}

			indent := strings.Repeat("  ", depth)
			if info.IsDir() {
				tree.WriteString(fmt.Sprintf("%s[%s/]\n", indent, info.Name()))
			} else {
				tree.WriteString(fmt.Sprintf("%s- %s\n", indent, info.Name()))
			}
			return nil
		})

		if err != nil {
			return ProjectStructureResults{Error: err.Error()}, nil
		}
		return ProjectStructureResults{Tree: tree.String()}, nil
	}

	mapDirTool, _ := functiontool.New(
		functiontool.Config{
			Name:        "map_project_structure",
			Description: "Generates a tree-view map of the project folders and main files to understand the architecture.",
		},
		mapDirFunc,
	)

	// 2. Initialize the Gemini Model
	model, err := gemini.NewModel(ctx, "gemini-2.0-flash", &genai.ClientConfig{})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize architecture model: %w", err)
	}

	// 3. Create the Architecture Agent
	return llmagent.New(llmagent.Config{
		Name:  "architecture_overview_agent",
		Model: model,
		Instruction: "You are a Senior Software Architect. Your goal is to explain the high-level design of a codebase. " +
			"Always start by mapping the project structure. Identify core packages, identify where the entry point is, " +
			"and explain the relationship between layers (e.g. API, Domain, Infrastructure).",
		Description: "An agent that provides strategic architectural overviews of a repository.",
		Tools:       []tool.Tool{mapDirTool},
	})
}
