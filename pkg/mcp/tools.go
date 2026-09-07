package mcp

import (
	"github.com/hangry-coder/bffx/pkg/compiler"
	"github.com/hangry-coder/bffx/pkg/deploy"
	"github.com/hangry-coder/bffx/pkg/generator"
	"github.com/hangry-coder/bffx/pkg/doctor"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func (s *Server) handleToolsList(req *Request) *Response {
	return &Response{
		JSONRPC: JSONRPCVersion,
		ID:      req.ID,
		Result: map[string]any{
			"tools": append(s.tools, Tool{
				Name:        "doctor.check",
				Description: "Run BFFX health checks (doctor)",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"strict": map[string]any{"type": "boolean", "description": "Fail on warnings"},
					},
				},
			}),
		},
	}
}

func (s *Server) handleToolsCall(req *Request) *Response {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return &Response{
			JSONRPC: JSONRPCVersion,
			ID:      req.ID,
			Error:   &Error{Code: -32602, Message: "Invalid params"},
		}
	}

	switch params.Name {
	case "manifest.list":
		return s.toolManifestList(req)
	case "manifest.read":
		return s.toolManifestRead(req, params.Arguments)
	case "graph.inspect":
		return s.toolGraphInspect(req)
	case "sync.dryRun":
		return s.toolSyncDryRun(req)
	case "resource.create":
		return s.toolResourceCreate(req, params.Arguments)
	case "sync.apply":
		return s.toolSyncApply(req)
	case "client.scaffold":
		return s.toolClientScaffold(req, params.Arguments)
	case "deploy.init":
		return s.toolDeployInit(req)
	case "deploy.ship":
		return s.toolDeployShip(req)
	case "doctor.check":
		return s.toolDoctorCheck(req, params.Arguments)
	default:
		return &Response{
			JSONRPC: JSONRPCVersion,
			ID:      req.ID,
			Error:   &Error{Code: -32601, Message: "Tool not found: " + params.Name},
		}
	}
}

// Tool Implementations

func (s *Server) toolManifestList(req *Request) *Response {
	data := make([]map[string]string, 0)
	for _, m := range s.reg.Resources {
		data = append(data, map[string]string{"kind": "Resource", "name": m.Metadata.Name, "path": m.Path})
	}
	for _, m := range s.reg.Builders {
		data = append(data, map[string]string{"kind": "Builder", "name": m.Metadata.Name, "path": m.Path})
	}
	return s.toolResult(req, data)
}

func (s *Server) toolManifestRead(req *Request, args json.RawMessage) *Response {
	var params struct {
		Path string `json:"path"`
	}
	json.Unmarshal(args, &params)

	if params.Path == "" {
		return s.toolError(req, "path is required")
	}

	// Safety: only allow reading within project root and likely manifest files
	cleanedRoot, err := filepath.Abs(filepath.Clean(s.root))
	if err != nil {
		return s.toolError(req, "invalid project root: "+err.Error())
	}
	fullPath, err := filepath.Abs(filepath.Clean(filepath.Join(cleanedRoot, params.Path)))
	if err != nil {
		return s.toolError(req, "invalid target path: "+err.Error())
	}
	if !strings.HasPrefix(fullPath, cleanedRoot+string(filepath.Separator)) && fullPath != cleanedRoot {
		return s.toolError(req, "access denied: path outside project root")
	}

	b, err := os.ReadFile(fullPath)
	if err != nil {
		return s.toolError(req, "failed to read file: "+err.Error())
	}

	return s.toolResult(req, map[string]string{"path": params.Path, "content": string(b)})
}

func (s *Server) toolGraphInspect(req *Request) *Response {
	content, err := s.readProjectFile(".bffx/graph.json")
	if err != nil {
		return s.toolError(req, "graph not found")
	}
	return s.toolResult(req, json.RawMessage(content))
}

func (s *Server) toolSyncDryRun(req *Request) *Response {
	output, err := s.captureOutput(func() error {
		_, err := compiler.Sync(s.root, true)
		return err
	})
	if err != nil {
		return s.toolError(req, "dry-run failed: "+err.Error())
	}
	return s.toolResult(req, map[string]string{"output": output})
}

func (s *Server) toolResourceCreate(req *Request, args json.RawMessage) *Response {
	var params struct {
		Name   string `json:"name"`
		Fields []struct {
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"fields"`
	}
	json.Unmarshal(args, &params)

	if params.Name == "" {
		return s.toolError(req, "name is required")
	}

	genFields := make([]generator.Field, len(params.Fields))
	for i, f := range params.Fields {
		genFields[i] = generator.Field{Name: f.Name, Type: f.Type}
	}

	// 1. Generate scaffold
	if err := generator.GenerateScaffold(s.root, params.Name, genFields, generator.ResourceOptions{WithHooks: true}); err != nil {

		return s.toolError(req, "generation failed: "+err.Error())
	}

	// 2. Sync to apply
	if _, err := compiler.Sync(s.root, false); err != nil {
		return s.toolError(req, "sync failed: "+err.Error())
	}

	return s.toolResult(req, map[string]string{"status": "ok", "message": "Resource '" + params.Name + "' created and synced."})
}

func (s *Server) toolSyncApply(req *Request) *Response {
	if _, err := compiler.Sync(s.root, false); err != nil {
		return s.toolError(req, "sync failed: "+err.Error())
	}
	return s.toolResult(req, map[string]string{"status": "ok", "message": "Sync complete."})
}

func (s *Server) toolClientScaffold(req *Request, args json.RawMessage) *Response {
	var params struct {
		Platform string `json:"platform"`
		Resource string `json:"resource"`
	}
	json.Unmarshal(args, &params)

	if params.Resource == "" {
		return s.toolError(req, "resource name is required")
	}

	// Generate SwiftUI View code block
	if params.Platform == "swiftui" {
		code := fmt.Sprintf(`// SwiftUI View for %s
import SwiftUI

struct %sListView: View {
    @State private var items: [%s] = []
    
    var body: some View {
        List(items) { item in
            HStack {
                VStack(alignment: .leading) {
                    Text(item.name ?? "Unknown")
                        .font(.headline)
                    Text(item.id)
                        .font(.caption)
                        .foregroundColor(.secondary)
                }
            }
        }
        .task {
            // items = try? await BFFXClient.shared.request("/%ss")
        }
    }
}
`, params.Resource, params.Resource, params.Resource, strings.ToLower(params.Resource))
		return s.toolResult(req, map[string]string{"code": code, "platform": "swiftui"})
	}

	return s.toolError(req, "unsupported platform: "+params.Platform)
}

func (s *Server) toolDeployInit(req *Request) *Response {
	workspaceRoot := deploy.DetectWorkspaceRoot(s.root)

	output, err := s.captureOutput(func() error {
		return deploy.Init(s.root, workspaceRoot, nil)
	})
	if err != nil {
		return s.toolError(req, "deploy init failed: "+err.Error())
	}
	return s.toolResult(req, map[string]string{"status": "ok", "message": "Deployment initialized", "output": output})
}

func (s *Server) toolDeployShip(req *Request) *Response {
	output, err := s.captureOutput(func() error {
		return deploy.Ship(s.root)
	})
	if err != nil {
		return s.toolError(req, "deploy ship failed: "+err.Error())
	}
	return s.toolResult(req, map[string]string{"status": "ok", "message": "Ship command executed", "output": output})
}

func (s *Server) toolDoctorCheck(req *Request, args json.RawMessage) *Response {
	results, err := doctor.Check(s.root)
	if err != nil {
		return s.toolError(req, "doctor failed: "+err.Error())
	}
	return s.toolResult(req, results)
}

// Helpers

func (s *Server) toolResult(req *Request, data any) *Response {
	var content []map[string]any
	if b, ok := data.(json.RawMessage); ok {
		content = []map[string]any{{"type": "text", "text": string(b)}}
	} else {
		b, _ := json.MarshalIndent(data, "", "  ")
		content = []map[string]any{{"type": "text", "text": string(b)}}
	}
	return &Response{
		JSONRPC: JSONRPCVersion,
		ID:      req.ID,
		Result: map[string]any{
			"content": content,
		},
	}
}

func (s *Server) toolError(req *Request, message string) *Response {
	return &Response{
		JSONRPC: JSONRPCVersion,
		ID:      req.ID,
		Result: map[string]any{
			"content": []map[string]any{{"type": "text", "text": "Error: " + message}},
			"isError": true,
		},
	}
}

func (s *Server) captureOutput(f func() error) (string, error) {
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stdout = w
	os.Stderr = w

	outChan := make(chan string)
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, r)
		outChan <- buf.String()
	}()

	err := f()
	
	w.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr
	
	return <-outChan, err
}
