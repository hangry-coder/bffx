package mcp

import (
	"github.com/hangry-coder/bffx/pkg/manifest"
	"sync"
)

// Server implements the Model Context Protocol (MCP).
type Server struct {
	mu        sync.RWMutex
	reg       *manifest.Registry
	root      string
	resources []Resource
	tools     []Tool
}

// NewServer creates a new MCP server for a given bffx project.
func NewServer(root string, reg *manifest.Registry) *Server {
	s := &Server{
		root: root,
		reg:  reg,
	}
	s.registerDefaultTools()
	s.registerDefaultResources()
	return s
}

func (s *Server) registerDefaultResources() {
	s.resources = []Resource{
		{URI: "bffx://project", Name: "Project Specification", Description: "The loaded Project manifest", MimeType: "application/json"},
		{URI: "bffx://graph", Name: "Project Graph", Description: "The compiled project dependency graph", MimeType: "application/json"},
		{URI: "bffx://manifests", Name: "Manifest Registry", Description: "List of all registered manifests", MimeType: "application/json"},
		{URI: "bffx://diagnostics", Name: "Diagnostics", Description: "Latest sync diagnostics", MimeType: "application/json"},
		{URI: "bffx://openapi", Name: "OpenAPI Spec", Description: "Auto-generated OpenAPI documentation", MimeType: "application/json"},
	}
}

func (s *Server) registerDefaultTools() {
	s.tools = []Tool{
		{
			Name:        "manifest.list",
			Description: "List all manifests in the project",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{},
			},
		},
		{
			Name:        "manifest.read",
			Description: "Read the raw content of a manifest file",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{"type": "string"},
				},
				"required": []string{"path"},
			},
		},
		{
			Name:        "graph.inspect",
			Description: "Inspect the compiled project graph",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{},
			},
		},
		{
			Name:        "sync.dryRun",
			Description: "Preview changes that would be applied by a sync",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{},
			},
		},
		{
			Name:        "resource.create",
			Description: "Generate a new resource manifest and sync",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{"type": "string"},
					"fields": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"name": map[string]any{"type": "string"},
								"type": map[string]any{"type": "string", "enum": []string{"string", "int", "float", "bool", "json"}},
							},
						},
					},
				},
				"required": []string{"name", "fields"},
			},
		},
		{
			Name:        "sync.apply",
			Description: "Perform a full project sync and code generation",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{},
			},
		},
		{
			Name:        "client.scaffold",
			Description: "Generate SwiftUI or Flutter client-side code for a resource",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"platform": map[string]any{"type": "string", "enum": []string{"swiftui", "flutter"}},
					"resource": map[string]any{"type": "string"},
				},
				"required": []string{"platform", "resource"},
			},
		},
		{
			Name:        "deploy.init",
			Description: "Initialize deployment configuration (Dockerfile, docker-compose.yml)",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{},
			},
		},
		{
			Name:        "deploy.ship",
			Description: "Build and deploy the project to the configured VPS",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{},
			},
		},
	}
}

