package main

import (
	"context"
    "log"

	"github.com/mark3labs/mcp-go/mcp"
	_ "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func helloHandler(
    ctx context.Context,
    req mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
    return mcp.NewToolResultText("hello mcp!"), nil
}

func main() {

    s := server.NewMCPServer(
        "mcp-n-server",
        "1.0.0",
    )

    tool := mcp.NewTool(
        "hello",
        mcp.WithDescription("挨拶を返します"),
    )

    s.AddTool(tool, helloHandler)

    if err := server.ServeStdio(s); err != nil {
        log.Fatal(err)
    }
}