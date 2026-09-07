package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func (s *Server) handleResourcesList(req *Request) *Response {
	return &Response{
		JSONRPC: JSONRPCVersion,
		ID:      req.ID,
		Result: map[string]any{
			"resources": s.resources,
		},
	}
}

func (s *Server) handleResourcesRead(req *Request) *Response {
	var params struct {
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return &Response{
			JSONRPC: JSONRPCVersion,
			ID:      req.ID,
			Error:   &Error{Code: -32602, Message: "Invalid params"},
		}
	}

	var content string
	var err error

	switch params.URI {
	case "bffx://project":
		b, _ := json.MarshalIndent(s.reg.Project, "", "  ")
		content = string(b)
	case "bffx://graph":
		content, err = s.readProjectFile(".bffx/graph.json")
	case "bffx://manifests":
		data := make([]map[string]string, 0)
		for _, m := range s.reg.Resources {
			data = append(data, map[string]string{"kind": "Resource", "name": m.Metadata.Name, "path": m.Path})
		}
		for _, m := range s.reg.Builders {
			data = append(data, map[string]string{"kind": "Builder", "name": m.Metadata.Name, "path": m.Path})
		}
		b, _ := json.MarshalIndent(data, "", "  ")
		content = string(b)
	case "bffx://diagnostics":
		content, err = s.readProjectFile(".bffx/diagnostics.json")
	case "bffx://openapi":
		content, err = s.readProjectFile(".bffx/openapi.json")
	default:
		return &Response{
			JSONRPC: JSONRPCVersion,
			ID:      req.ID,
			Error:   &Error{Code: -32602, Message: "Resource not found: " + params.URI},
		}
	}

	if err != nil {
		return &Response{
			JSONRPC: JSONRPCVersion,
			ID:      req.ID,
			Error:   &Error{Code: -32000, Message: "Read error: " + err.Error()},
		}
	}

	return &Response{
		JSONRPC: JSONRPCVersion,
		ID:      req.ID,
		Result: map[string]any{
			"contents": []map[string]any{
				{
					"uri":      params.URI,
					"mimeType": "application/json",
					"text":     content,
				},
			},
		},
	}
}

func (s *Server) readProjectFile(relPath string) (string, error) {
	path := filepath.Join(s.root, relPath)
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
