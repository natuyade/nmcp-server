package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const maxFileSize = 1 * 1024 * 1024

var waitTimes = map[string]int {
    "fast": 1,
    "normal": 3,
    "slow": 5,
}

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

    mode, err := request.RequireString("mode")
    if err != nil {
        return mcp.NewToolResultError(err.Error()), nil
    }

    seconds, ok := waitTimes[mode]
    if !ok {
        message := fmt.Sprintf("Invalid mode: %s", mode)
        return mcp.NewToolResultError(message), nil
    }

    timer := time.NewTimer(time.Duration(seconds) * time.Second)
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

func readFileHandler(
    ctx context.Context,
    request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
    path, err := request.RequireString("path")
    if err != nil {
        return mcp.NewToolResultError(err.Error()), nil
    }

    rootDir, err := os.Getwd()
    if err != nil {
        return mcp.NewToolResultError(err.Error()), nil
    }

    root, err := os.OpenRoot(rootDir)
    if err != nil {
        return mcp.NewToolResultError(err.Error()), nil
    }
    defer root.Close()

    info, err := root.Stat(path)
    if err != nil {
        return mcp.NewToolResultError(err.Error()), nil
    }

    if info.Size() > maxFileSize {
        return mcp.NewToolResultError("ファイルサイズが大きすぎます"), nil
    }

    data, err := root.ReadFile(path)
    if err != nil {
        return mcp.NewToolResultError(err.Error()), nil
    }

    // utf-8 検証
    if !utf8.Valid(data) {
        return mcp.NewToolResultError("UTF-8テキストではありません"), nil
    }
    
    return mcp.NewToolResultText(string(data)), nil
}

func listFilesHandler(
    ctx context.Context,
    request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	path, err := request.RequireString("path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	rootDir, err := os.Getwd()
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	root, err := os.OpenRoot(rootDir)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	defer root.Close()
    
    // .Open()は指定されたpathの*os.File(操作するためのハンドラ)を取得する
    dir, err := root.Open(path)
    if err != nil {
        return mcp.NewToolResultError(err.Error()), nil
    }

    // .ReadDir()でディレクトリ内の各ファイルの情報を取得する
    entries, err := dir.ReadDir(-1)
    if err != nil {
        return mcp.NewToolResultError(err.Error()), nil
    }

    result := []string{}

    for _, entry := range entries {
        name := entry.Name()

        if entry.IsDir() {
            name += "/"
        }
        result = append(result, name)
    }

    if len(result) == 0 {
        return mcp.NewToolResultError("empty directory"), nil
    }

    // stringsで一つの文字列に, .Joinで文字の間に指定した文字を入れる
    text := strings.Join(result, "\n")

    return mcp.NewToolResultText(text), nil
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
		mcp.WithDescription("Modeに応じた時間経過後、doneを返します"),
        mcp.WithString(
            "mode",
            mcp.Required(),
            mcp.Description("待機モード"),
            mcp.Enum("fast", "normal", "slow"),
        ),
	)

    readFileTool := mcp.NewTool(
        "read_text_file",
        mcp.WithDescription("project内のtextファイルを読み取ります"),
        mcp.WithString(
            "path",
            mcp.Required(),
            mcp.Description("ファイルパス"),
        ),
    )

    listFilesTool := mcp.NewTool(
        "lest_files",
        mcp.WithDescription("指定したディレクトリのファイル一覧を取得します"),
        mcp.WithString(
            "path",
            mcp.Required(),
            mcp.Description("一覧を取得するディレクトリ"),
        ),
    )

	// ツールとハンドラーをサーバーに登録
	s.AddTool(helloTool, helloHandler)
	s.AddTool(waitTool, waitHandler)
	s.AddTool(readFileTool, readFileHandler)
	s.AddTool(listFilesTool, listFilesHandler)

	if err := server.ServeStdio(s); err != nil {
		log.Fatal(err)
	}
}
