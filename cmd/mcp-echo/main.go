package main

import (
	"context"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type echoIn struct {
	Message string `json:"message" jsonschema:"text to echo back"`
}

type echoOut struct {
	Echo string `json:"echo"`
}

func echo(_ context.Context, _ *mcp.CallToolRequest, in echoIn) (*mcp.CallToolResult, echoOut, error) {
	msg := in.Message
	if msg == "" {
		msg = "(empty)"
	}
	return nil, echoOut{Echo: msg}, nil
}

func main() {
	server := mcp.NewServer(&mcp.Implementation{Name: "buatpostingan-echo", Version: "1.0.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "echo",
		Description: "Echo a message back (read-only sample MCP tool).",
	}, echo)
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
