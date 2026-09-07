package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"github.com/hangry-coder/bffx/pkg/logger"
	"os"
)

// ServeStdio starts the MCP server using standard input and output as the transport.
func (s *Server) ServeStdio() {
	reader := bufio.NewReader(os.Stdin)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				return
			}
			logger.Error("mcp read error: %v", err)
			continue
		}

		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			s.sendError(os.Stdout, nil, -32700, "Parse error: "+err.Error())
			continue
		}

		resp := s.handleRequest(&req)
		if resp != nil {
			s.sendResponse(os.Stdout, resp)
		}
	}
}

func (s *Server) handleRequest(req *Request) *Response {
	// Protocol initialization
	if req.Method == "initialize" {
		return &Response{
			JSONRPC: JSONRPCVersion,
			ID:      req.ID,
			Result: map[string]any{
				"protocolVersion": "2024-11-05",
				"capabilities": map[string]any{
					"resources": map[string]any{},
					"tools":     map[string]any{},
				},
				"serverInfo": map[string]any{
					"name":    "bffx",
					"version": "1.0.0-alpha",
				},
			},
		}
	}

	// Dispatch by method
	switch req.Method {
	case "resources/list":
		return s.handleResourcesList(req)
	case "resources/read":
		return s.handleResourcesRead(req)
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(req)
	case "notifications/initialized":
		return nil // NOP
	default:
		return &Response{
			JSONRPC: JSONRPCVersion,
			ID:      req.ID,
			Error: &Error{
				Code:    -32601,
				Message: fmt.Sprintf("Method not found: %s", req.Method),
			},
		}
	}
}

func (s *Server) sendResponse(w io.Writer, resp *Response) {
	b, _ := json.Marshal(resp)
	fmt.Fprintf(w, "%s\n", b)
}

func (s *Server) sendError(w io.Writer, id any, code int, message string) {
	resp := &Response{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Error: &Error{
			Code:    code,
			Message: message,
		},
	}
	s.sendResponse(w, resp)
}
