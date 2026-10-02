package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const maxFileSize = 1 * 1024 * 1024

var waitTimes = map[string]int{
	"fast":   1,
	"normal": 3,
	"slow":   5,
}

var httpClient = &http.Client{}

type DelegateRequest struct {
	Role string `json:"role"`
	Task string `json:"task"`
}
type DelegateResponse struct {
	Result string `json:"result"`
}

func helloHandler(
	// context.Contextは、キャンセルやタイムアウトなどの情報を伝播させられる
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
	defer dir.Close()

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
		return mcp.NewToolResultText("empty directory"), nil
	}

	// stringsで一つの文字列に, .Joinで文字の間に指定した文字を入れる
	text := strings.Join(result, "\n")

	return mcp.NewToolResultText(text), nil
}

func delegateHandler(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	// mcp引数取得
	role, err := request.RequireString("role")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	task, err := request.RequireString("task")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// requestを作成
	delegateRequest := DelegateRequest{
		Role: role,
		Task: task,
	}

	delegateResponse, err := delegateToLLM(ctx, delegateRequest)
	if err != nil {
        // errors.Isでerrがwrapされている場合でも, 元のエラーを判定できる
		switch {
		case errors.Is(err, context.Canceled):
			return mcp.NewToolResultError("request canceled"), nil
		case errors.Is(err, context.DeadlineExceeded):
			return mcp.NewToolResultError("request timed out"), nil
		default:
			return mcp.NewToolResultError(err.Error()), nil
		}
	}

	// delegateResponseを返す
	return mcp.NewToolResultText(delegateResponse.Result), nil
}

func delegateToLLM(
	ctx context.Context,
	delegateRequest DelegateRequest,
) (DelegateResponse, error) {

    ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
    defer cancel()

	// jsonにMarshal(変換)
	body, err := json.Marshal(delegateRequest)
	if err != nil {
		return DelegateResponse{}, fmt.Errorf("marshal request: %w", err)
	}

	// HTTPリクエストを作成
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		"http://127.0.0.1:8081/delegate",
		bytes.NewReader(body),
	)
	if err != nil {
		return DelegateResponse{}, fmt.Errorf("create HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Doのerrは404などのHTTPステータスコードが
	// 200番台でないときなどのエラーではなく,
	// ネットワークエラーなどの通信に関するエラーを返す

	// HTTPリクエストをPOST
	resp, err := httpClient.Do(req)
	if err != nil {
		return DelegateResponse{}, fmt.Errorf("send HTTP request: %w", err)
	}
	// responseを読み取り後, 呼び出し側でClose()する必要があるのでdeferでClose()を呼ぶ
	defer resp.Body.Close()

	// HTTP status確認
	// HTTPステータスコードが200番台でない場合はエラーとして返す
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return DelegateResponse{},
			fmt.Errorf("dummy server returned HTTP %d", resp.StatusCode)
	}

	var delegateResponse DelegateResponse
	// responseをjsonデコード
	if err := json.NewDecoder(resp.Body).Decode(&delegateResponse); err != nil {
		return DelegateResponse{}, fmt.Errorf("decode response: %w", err)
	}

	return delegateResponse, nil
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
		"list_files",
		mcp.WithDescription("指定したディレクトリのファイル一覧を取得します"),
		mcp.WithString(
			"path",
			mcp.Required(),
			mcp.Description("一覧を取得するディレクトリ"),
		),
	)

	delegateTool := mcp.NewTool(
		"delegate_task",
		mcp.WithDescription("別のLLMにタスクを委任します"),
		mcp.WithString(
			"role",
			mcp.Required(),
			mcp.Description("委任先の役割"),
		),
		mcp.WithString(
			"task",
			mcp.Required(),
			mcp.Description("委任するタスク"),
		),
	)

	// ツールとハンドラーをサーバーに登録
	s.AddTool(helloTool, helloHandler)
	s.AddTool(waitTool, waitHandler)
	s.AddTool(readFileTool, readFileHandler)
	s.AddTool(listFilesTool, listFilesHandler)
	s.AddTool(delegateTool, delegateHandler)

	if err := server.ServeStdio(s); err != nil {
		log.Fatal(err)
	}
}
