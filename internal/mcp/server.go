package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// Server is the MCP JSON-RPC server over stdio.
type Server struct {
	reader  *bufio.Reader
	writer  io.Writer
	tools   *ToolRegistry
}

// NewServer creates a new MCP server.
func NewServer(tools *ToolRegistry) *Server {
	return &Server{
		reader: bufio.NewReader(os.Stdin),
		writer: os.Stdout,
		tools:  tools,
	}
}

// Run starts the stdio JSON-RPC loop. Blocks until EOF.
func (s *Server) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line, err := s.reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("read input: %w", err)
		}

		var req JSONRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			s.writeError(nil, ErrCodeParse, "Parse error: "+err.Error())
			continue
		}

		resp := s.handleRequest(ctx, &req)
		s.writeResponse(resp)
	}
}

func (s *Server) handleRequest(ctx context.Context, req *JSONRPCRequest) JSONRPCResponse {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(ctx, req)
	default:
		return NewErrorResponse(req.ID, ErrCodeMethod, "Method not found: "+req.Method, nil)
	}
}

func (s *Server) handleInitialize(req *JSONRPCRequest) JSONRPCResponse {
	result := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"serverInfo": map[string]string{
			"name":    "skillvault",
			"version": "v3",
		},
		"capabilities": map[string]interface{}{
			"tools": map[string]bool{},
		},
	}
	return NewResult(req.ID, result)
}

func (s *Server) handleToolsList(req *JSONRPCRequest) JSONRPCResponse {
	tools := s.tools.List()
	result := map[string]interface{}{
		"tools": tools,
	}
	return NewResult(req.ID, result)
}

func (s *Server) handleToolsCall(ctx context.Context, req *JSONRPCRequest) JSONRPCResponse {
	var params ToolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeParams, "Invalid params", err.Error())
	}

	result, err := s.tools.Call(ctx, params.Name, params.Arguments)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeAppDomain, err.Error(), nil)
	}

	return NewResult(req.ID, result)
}

func (s *Server) writeResponse(resp JSONRPCResponse) {
	data, _ := json.Marshal(resp)
	fmt.Fprintf(s.writer, "%s\n", data)
}

func (s *Server) writeError(id interface{}, code int, message string) {
	resp := NewErrorResponse(id, code, message, nil)
	data, _ := json.Marshal(resp)
	fmt.Fprintf(s.writer, "%s\n", data)
}
