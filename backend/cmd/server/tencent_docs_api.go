package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var tencentDocsAPIBaseURL = "https://docs.qq.com"

type tencentDocsEnvelope struct {
	Ret  int    `json:"ret"`
	Msg  string `json:"msg"`
	Data struct {
		FileID string `json:"fileID"`
		Title  string `json:"title"`
	} `json:"data"`
}

type tencentDocsNode struct {
	Type     string            `json:"type"`
	Text     string            `json:"text"`
	Children []tencentDocsNode `json:"children"`
}

type tencentDocsDocumentResponse struct {
	Document *tencentDocsNode `json:"document"`
	Version  int              `json:"version"`
	Code     int              `json:"code"`
	Message  string           `json:"message"`
}

func (rt *Runtime) parseTencentDoc(ctx context.Context, rawURL string) (ParseResult, error) {
	cfg, db, _ := rt.deps()
	auth, err := loadTencentDocsAuth(db, cfg)
	if err != nil {
		return ParseResult{}, fmt.Errorf("读取腾讯文档凭据失败: %w", err)
	}
	if auth == nil {
		return parsePublicTencentDoc(ctx, rawURL)
	}
	if auth.ExpiresAt != nil && !time.Now().Before(auth.ExpiresAt.Add(-30*time.Second)) {
		return ParseResult{}, errors.New("腾讯文档 Access Token 已过期，请在站点设置中更新")
	}
	return parseTencentDocOpenAPI(ctx, rawURL, *auth)
}

func parseTencentDocOpenAPI(ctx context.Context, rawURL string, auth tencentDocsAuth) (ParseResult, error) {
	encodedID, err := extractDocID(rawURL)
	if err != nil {
		return ParseResult{}, err
	}

	converterURL, err := url.Parse(tencentDocsAPIBaseURL + "/openapi/drive/v2/util/converter")
	if err != nil {
		return ParseResult{}, err
	}
	query := converterURL.Query()
	query.Set("type", "2")
	query.Set("value", encodedID)
	converterURL.RawQuery = query.Encode()
	converterBody, err := requestTencentDocs(ctx, auth, http.MethodGet, converterURL.String())
	if err != nil {
		return ParseResult{}, fmt.Errorf("转换腾讯文档 ID 失败: %w", err)
	}
	var converter tencentDocsEnvelope
	if err := json.Unmarshal(converterBody, &converter); err != nil {
		return ParseResult{}, fmt.Errorf("解析腾讯文档 ID 转换结果失败: %w", err)
	}
	if converter.Ret != 0 || converter.Data.FileID == "" {
		return ParseResult{}, fmt.Errorf("转换腾讯文档 ID 失败: %s", fallbackTencentMessage(converter.Msg))
	}

	fileID := escapeTencentDocsFileID(converter.Data.FileID)
	title := ""
	metadataBody, metadataErr := requestTencentDocs(ctx, auth, http.MethodGet, tencentDocsAPIBaseURL+"/openapi/drive/v2/files/"+fileID+"/metadata")
	if metadataErr == nil {
		var metadata tencentDocsEnvelope
		if json.Unmarshal(metadataBody, &metadata) == nil && metadata.Ret == 0 {
			title = metadata.Data.Title
		}
	}

	documentBody, err := requestTencentDocs(ctx, auth, http.MethodGet, tencentDocsAPIBaseURL+"/openapi/doc/v3/"+fileID)
	if err != nil {
		return ParseResult{}, fmt.Errorf("获取腾讯文档内容失败: %w", err)
	}
	var response tencentDocsDocumentResponse
	if err := json.Unmarshal(documentBody, &response); err != nil {
		return ParseResult{}, fmt.Errorf("解析腾讯文档内容失败: %w", err)
	}
	if response.Document == nil {
		return ParseResult{}, fmt.Errorf("腾讯文档内容接口返回错误: %s", fallbackTencentMessage(response.Message))
	}
	var text strings.Builder
	appendTencentDocsNodeText(&text, *response.Document)
	items := extractFontItemsFromText(text.String())
	if len(items) == 0 {
		return ParseResult{}, errors.New("未解析到字体下载链接")
	}
	return ParseResult{Title: title, Items: items}, nil
}

func requestTencentDocs(ctx context.Context, auth tencentDocsAuth, method, target string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Access-Token", auth.AccessToken)
	req.Header.Set("Client-Id", auth.ClientID)
	req.Header.Set("Open-Id", auth.OpenID)
	req.Header.Set("Accept", "application/json")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var payload struct {
			Message string `json:"message"`
			Msg     string `json:"msg"`
		}
		_ = json.Unmarshal(body, &payload)
		message := payload.Message
		if message == "" {
			message = payload.Msg
		}
		if message == "" {
			message = http.StatusText(resp.StatusCode)
		}
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, message)
	}
	return body, nil
}

func appendTencentDocsNodeText(output *strings.Builder, node tencentDocsNode) {
	if node.Text != "" {
		output.WriteString(node.Text)
	}
	for _, child := range node.Children {
		appendTencentDocsNodeText(output, child)
	}
}

func escapeTencentDocsFileID(fileID string) string {
	return strings.ReplaceAll(url.PathEscape(fileID), "$", "%24")
}

func fallbackTencentMessage(message string) string {
	if strings.TrimSpace(message) == "" {
		return "接口未返回详细原因"
	}
	return message
}
