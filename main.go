package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func helloHandler(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	// requestから引数を取得する
	name, err := request.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	age, err := request.RequireInt("age")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	message := fmt.Sprintf("hello %s! You are %d years old.", name, age)

	// 文字列をMCP Toolの実行結果として返す
	return mcp.NewToolResultText(message), nil
}

func waitHandler(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	seconds, err := request.RequireInt("seconds")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

    if seconds < 1 {
        return mcp.NewToolResultError("入力値は1以上でなければいけません"), nil
    }

	timer := time.NewTimer(time.Duration(seconds) * time.Second)
	//  deferはこの関数が終了するときに実行される
	defer timer.Stop()

    // <- はchannelから値 通知を受信する
    // selectは受信可能になったcaseの処理を実行する
	select {
	case <-ctx.Done():
		return mcp.NewToolResultError(ctx.Err().Error()), nil
	case <-timer.C:
		return mcp.NewToolResultText("done"), nil
	}
}

func main() {

	// サーバー本体
	s := server.NewMCPServer(
		"mcp-n-server",
		"1.0.0",
	)

	// clientから呼び出すツール
	helloTool := mcp.NewTool(
		// ツール名
		"hello",
		// ツールの説明(細かい仕様まで書いた方がいい、と思う)
		mcp.WithDescription("挨拶を返します"),
		// ツールの入力仕様を定義
		mcp.WithString(
			// 引数名
			"name",
			// 引数を必須にする
			mcp.Required(),
			// 引数の説明
			mcp.Description("名前欄"),
		),
		mcp.WithInteger(
			"age",
			mcp.Required(),
			mcp.Description("年齢"),
		),
	)

	waitTool := mcp.NewTool(
		"wait",
		mcp.WithDescription("指定された秒数後doneを返します"),
		mcp.WithInteger(
			"seconds",
			mcp.Required(),
            mcp.Min(1),
			mcp.Description("待機時間"),
		),
	)

	// ツールとハンドラーをサーバーに登録
	s.AddTool(helloTool, helloHandler)
	s.AddTool(waitTool, waitHandler)

	if err := server.ServeStdio(s); err != nil {
		log.Fatal(err)
	}
}
