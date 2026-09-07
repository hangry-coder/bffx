package mcp

import (
	"encoding/json"
	"testing"

	"github.com/hangry-coder/bffx/pkg/manifest"
)

func TestMCPServer_initialize(t *testing.T) {
	s := NewServer(".", &manifest.Registry{})
	req := &Request{JSONRPC: "2.0", ID: 1, Method: "initialize"}
	resp := s.handleRequest(req)
	if resp == nil || resp.Error != nil {
		t.Fatalf("initialize: %+v", resp)
	}
	raw, _ := json.Marshal(resp.Result)
	if !json.Valid(raw) {
		t.Fatal("invalid JSON result")
	}
	var out struct {
		ProtocolVersion string `json:"protocolVersion"`
		ServerInfo      struct {
			Name string `json:"name"`
		} `json:"serverInfo"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if out.ProtocolVersion == "" || out.ServerInfo.Name == "" {
		t.Fatalf("unexpected initialize payload: %s", string(raw))
	}
}

func TestMCPServer_toolsList(t *testing.T) {
	s := NewServer(".", &manifest.Registry{})
	req := &Request{JSONRPC: "2.0", ID: "x", Method: "tools/list"}
	resp := s.handleRequest(req)
	if resp == nil || resp.Error != nil {
		t.Fatalf("tools/list: %+v", resp)
	}
	m, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("result type %T", resp.Result)
	}
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Tools []json.RawMessage `json:"tools"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Tools) < 2 {
		t.Fatalf("expected at least 2 tools, got %d", len(out.Tools))
	}
}

func TestMCPServer_unknownMethod(t *testing.T) {
	s := NewServer(".", &manifest.Registry{})
	resp := s.handleRequest(&Request{JSONRPC: "2.0", ID: 1, Method: "does/not/exist"})
	if resp == nil || resp.Error == nil {
		t.Fatalf("expected JSON-RPC error, got %+v", resp)
	}
}

func TestMCPServer_toolsCall_manifestList(t *testing.T) {
	s := NewServer(t.TempDir(), &manifest.Registry{})
	params, _ := json.Marshal(map[string]any{"name": "manifest.list", "arguments": map[string]any{}})
	resp := s.handleRequest(&Request{JSONRPC: "2.0", ID: 9, Method: "tools/call", Params: params})
	if resp == nil || resp.Error != nil {
		t.Fatalf("manifest.list: %+v", resp)
	}
}

func TestMCPServer_toolsCall_unknownTool(t *testing.T) {
	s := NewServer(t.TempDir(), &manifest.Registry{})
	params, _ := json.Marshal(map[string]any{"name": "nope.nope", "arguments": map[string]any{}})
	resp := s.handleRequest(&Request{JSONRPC: "2.0", ID: 2, Method: "tools/call", Params: params})
	if resp == nil || resp.Error == nil {
		t.Fatalf("expected error for unknown tool, got %+v", resp)
	}
}
