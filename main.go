package main

import (
	"context"
    "log"
    "fmt"

	"github.com/mark3labs/mcp-go/mcp"
	_ "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func helloHandler(
    ctx context.Context,
    request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
    // 引数の取得
    name, err := request.RequireString("name")
    if err != nil {
        return mcp.NewToolResultError(err.Error()), nil
    }

    message := fmt.Sprintf("hello %s!", name)

    // Mcp用のString型(mcp.CallToolResult)に変換して返す
    return mcp.NewToolResultText(message), nil
}

func main() {

    // サーバー本体
    s := server.NewMCPServer(
        "mcp-n-server",
        "1.0.0",
    )

    // clientから呼び出すツール
    tool := mcp.NewTool(
        "hello",
        // ツールの説明(細かい仕様まで書いた方がいい、と思う)
        mcp.WithDescription("挨拶を返します"),
        // ツールの引数
        mcp.WithString(
            // 引数名
            "name",
            // 引数を必須にする
            mcp.Required(),
            // 引数の説明
            mcp.Description("挨拶相手の名前"),
        ),
    )

    // ツールとハンドラーをサーバーに登録
    s.AddTool(tool, helloHandler)

    if err := server.ServeStdio(s); err != nil {
        log.Fatal(err)
    }
}