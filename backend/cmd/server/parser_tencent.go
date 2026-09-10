package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"google.golang.org/protobuf/encoding/protowire"
)

type ParsedFont struct {
	FontName    string
	DownloadURL string
	AccessCode  string
}

type ParseResult struct {
	Title string
	Items []ParsedFont
}

var (
	docURLRe       = regexp.MustCompile(`^https?://docs\.qq\.com/doc/([A-Za-z0-9_-]+)`)
	preloadRe      = regexp.MustCompile(`(?i)href=["']([^"']*dop-api/opendoc[^"']*)["']`)
	hyperlinkRe    = regexp.MustCompile(`HYPERLINK\s+(https?://[^\s\\]+)`)
	plainURLRe     = regexp.MustCompile(`^(https?://[^\s\\]+)`)
	accessCodeRe   = regexp.MustCompile(`(?:\\t|\t)?dkey\s+([A-Za-z0-9_-]+)`)
	callbackWrapRe = regexp.MustCompile(`^[A-Za-z0-9_$.]+\((?s:(.*))\);?$`)
)

func parsePublicTencentDoc(ctx context.Context, rawURL string) (ParseResult, error) {
	docID, err := extractDocID(rawURL)
	if err != nil {
		return ParseResult{}, err
	}
	page, err := fetchText(ctx, rawURL, rawURL)
	if err != nil {
		return ParseResult{}, fmt.Errorf("获取腾讯文档页面失败: %w", err)
	}
	openURL := findOpenDocURL(page, docID)
	body, err := fetchText(ctx, openURL, rawURL)
	if err != nil {
		return ParseResult{}, fmt.Errorf("获取腾讯文档数据失败: %w", err)
	}
	data, err := decodeOpenDocJSONP(body)
	if err != nil {
		return ParseResult{}, err
	}
	commands := data.initialCommands()
	extraCommands, err := fetchRemainingChunks(ctx, data, rawURL)
	if err != nil {
		return ParseResult{}, err
	}
	commands = append(commands, extraCommands...)
	title := data.ClientVars.Title
	if title == "" {
		title = data.ClientVars.InitialTitle
	}
	items := extractFontItems(commands)
	if len(items) == 0 {
		return ParseResult{}, errors.New("未解析到字体下载链接")
	}
	return ParseResult{Title: title, Items: items}, nil
}

func extractDocID(raw string) (string, error) {
	match := docURLRe.FindStringSubmatch(strings.TrimSpace(raw))
	if len(match) != 2 {
		return "", errors.New("请输入腾讯文档链接，例如 https://docs.qq.com/doc/...")
	}
	return match[1], nil
}

func fetchText(ctx context.Context, target, referer string) (string, error) {
	client := &http.Client{Timeout: 25 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/json,text/javascript,*/*")
	req.Header.Set("Referer", referer)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	bytes, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func findOpenDocURL(page, docID string) string {
	if match := preloadRe.FindStringSubmatch(page); len(match) == 2 {
		link := html.UnescapeString(match[1])
		if strings.HasPrefix(link, "//") {
			return "https:" + link
		}
		if strings.HasPrefix(link, "/") {
			return "https://docs.qq.com" + link
		}
		return link
	}
	params := url.Values{}
	params.Set("u", "")
	params.Set("id", docID)
	params.Set("normal", "1")
	params.Set("outformat", "1")
	params.Set("noEscape", "1")
	params.Set("commandsFormat", "1")
	params.Set("doc_chunk_version", "3")
	params.Set("preview_token", "")
	params.Set("doc_chunk_flag", "1")
	params.Set("callback", "clientVarsCallback")
	params.Set("xsrf", "")
	params.Set("t", fmt.Sprintf("%d", time.Now().UnixMilli()))
	return "https://docs.qq.com/dop-api/opendoc?" + params.Encode()
}

type openDocPayload struct {
	ClientVars struct {
		GlobalPadID  string `json:"globalPadId"`
		Title        string `json:"title"`
		InitialTitle string `json:"initialTitle"`
		Collab       struct {
			Rev                   int `json:"rev"`
			InitialAttributedText struct {
				Text []struct {
					Total        int `json:"total"`
					ChunkVersion int `json:"chunk_version"`
					Chunks       []struct {
						Index   int    `json:"index"`
						Command string `json:"command"`
					} `json:"chunks"`
				} `json:"text"`
			} `json:"initialAttributedText"`
		} `json:"collab_client_vars"`
	} `json:"clientVars"`
}

func decodeOpenDocJSONP(body string) (openDocPayload, error) {
	trimmed := strings.TrimSpace(body)
	if match := callbackWrapRe.FindStringSubmatch(trimmed); len(match) == 2 {
		trimmed = match[1]
	} else if idx := strings.Index(trimmed, "("); idx >= 0 {
		last := strings.LastIndex(trimmed, ")")
		if last > idx {
			trimmed = trimmed[idx+1 : last]
		}
	}
	var data openDocPayload
	if err := json.Unmarshal([]byte(trimmed), &data); err != nil {
		return data, fmt.Errorf("解析腾讯文档 JSONP 失败: %w", err)
	}
	return data, nil
}

func (data openDocPayload) initialCommands() []string {
	var commands []string
	for _, text := range data.ClientVars.Collab.InitialAttributedText.Text {
		for _, chunk := range text.Chunks {
			if chunk.Command != "" {
				commands = append(commands, chunk.Command)
			}
		}
	}
	return commands
}

type chunkPayload struct {
	Data struct {
		Total        int `json:"total"`
		ChunkVersion int `json:"chunk_version"`
		Chunks       []struct {
			Index   int    `json:"index"`
			Command string `json:"command"`
		} `json:"chunks"`
	} `json:"data"`
	RetCode int    `json:"retcode"`
	Message string `json:"errmsg"`
}

func fetchRemainingChunks(ctx context.Context, data openDocPayload, referer string) ([]string, error) {
	if data.ClientVars.GlobalPadID == "" || len(data.ClientVars.Collab.InitialAttributedText.Text) == 0 {
		return nil, nil
	}
	text := data.ClientVars.Collab.InitialAttributedText.Text[0]
	total := text.Total
	if total <= 0 {
		return nil, nil
	}
	maxIndex := -1
	for _, chunk := range text.Chunks {
		if chunk.Index > maxIndex {
			maxIndex = chunk.Index
		}
	}
	if maxIndex+1 >= total {
		return nil, nil
	}

	var commands []string
	for start := maxIndex + 1; start < total; start += 20 {
		count := 20
		if total-start < count {
			count = total - start
		}
		params := url.Values{}
		params.Set("padId", data.ClientVars.GlobalPadID)
		params.Set("commandsFormat", "1")
		params.Set("start_index", fmt.Sprintf("%d", start))
		params.Set("count", fmt.Sprintf("%d", count))
		params.Set("revision_version", fmt.Sprintf("%d", data.ClientVars.Collab.Rev))
		params.Set("doc_chunk_version", fmt.Sprintf("%d", text.ChunkVersion))
		body, err := fetchText(ctx, "https://docs.qq.com/dop-api/get/doc?"+params.Encode(), referer)
		if err != nil {
			return nil, fmt.Errorf("获取腾讯文档第 %d 批 chunk 失败: %w", start, err)
		}
		var payload chunkPayload
		if err := json.Unmarshal([]byte(body), &payload); err != nil {
			return nil, fmt.Errorf("解析腾讯文档第 %d 批 chunk 失败: %w", start, err)
		}
		if payload.RetCode != 0 {
			return nil, fmt.Errorf("腾讯文档 chunk 接口返回错误: %s", payload.Message)
		}
		if len(payload.Data.Chunks) == 0 {
			break
		}
		for _, chunk := range payload.Data.Chunks {
			if chunk.Command != "" {
				commands = append(commands, chunk.Command)
			}
		}
	}
	return commands, nil
}

func extractFontItems(commands []string) []ParsedFont {
	var decoded strings.Builder
	for _, command := range commands {
		raw, err := base64.StdEncoding.DecodeString(command)
		if err != nil {
			raw, err = base64.RawStdEncoding.DecodeString(command)
		}
		if err != nil {
			continue
		}
		if text, ok := extractTencentCommandText(raw); ok {
			decoded.Write(text)
			continue
		}
		decoded.Write(raw)
	}
	return extractFontItemsFromText(decoded.String())
}

func extractFontItemsFromText(text string) []ParsedFont {
	lines := regexp.MustCompile(`[\r\n]+`).Split(text, -1)
	seen := map[string]bool{}
	var items []ParsedFont
	lastText := ""
	for _, line := range lines {
		clean := cleanDecodedLine(line)
		if clean == "" {
			continue
		}
		if link := extractLanzouURL(clean); link != "" {
			if lastText == "" {
				continue
			}
			fontName := lastText
			lastText = ""
			code := ""
			if codeMatch := accessCodeRe.FindStringSubmatch(clean); len(codeMatch) == 2 {
				code = codeMatch[1]
			}
			key := fontName + "\x00" + link
			if seen[key] {
				continue
			}
			seen[key] = true
			items = append(items, ParsedFont{FontName: fontName, DownloadURL: link, AccessCode: code})
			continue
		}
		if isLikelyFontLine(clean) {
			lastText = clean
		}
	}
	return items
}

func extractTencentCommandText(raw []byte) ([]byte, bool) {
	var text bytes.Buffer
	rest := raw
	for len(rest) > 0 {
		number, wireType, tagLength := protowire.ConsumeTag(rest)
		if tagLength < 0 {
			return nil, false
		}
		rest = rest[tagLength:]
		if number == 2 && wireType == protowire.BytesType {
			operation, fieldLength := protowire.ConsumeBytes(rest)
			if fieldLength < 0 {
				return nil, false
			}
			if value, ok := extractTencentInsertText(operation); ok {
				text.Write(value)
			}
			rest = rest[fieldLength:]
			continue
		}
		fieldLength := protowire.ConsumeFieldValue(number, wireType, rest)
		if fieldLength < 0 {
			return nil, false
		}
		rest = rest[fieldLength:]
	}
	return text.Bytes(), text.Len() > 0
}

func extractTencentInsertText(operation []byte) ([]byte, bool) {
	var operationType uint64
	var insertPayload []byte
	rest := operation
	for len(rest) > 0 {
		number, wireType, tagLength := protowire.ConsumeTag(rest)
		if tagLength < 0 {
			return nil, false
		}
		rest = rest[tagLength:]
		switch {
		case number == 1 && wireType == protowire.VarintType:
			value, fieldLength := protowire.ConsumeVarint(rest)
			if fieldLength < 0 {
				return nil, false
			}
			operationType = value
			rest = rest[fieldLength:]
		case number == 6 && wireType == protowire.BytesType:
			value, fieldLength := protowire.ConsumeBytes(rest)
			if fieldLength < 0 {
				return nil, false
			}
			insertPayload = value
			rest = rest[fieldLength:]
		default:
			fieldLength := protowire.ConsumeFieldValue(number, wireType, rest)
			if fieldLength < 0 {
				return nil, false
			}
			rest = rest[fieldLength:]
		}
	}
	if operationType != 1 || len(insertPayload) == 0 {
		return nil, false
	}
	return extractTencentInsertPayload(insertPayload)
}

func extractTencentInsertPayload(payload []byte) ([]byte, bool) {
	rest := payload
	for len(rest) > 0 {
		number, wireType, tagLength := protowire.ConsumeTag(rest)
		if tagLength < 0 {
			return nil, false
		}
		rest = rest[tagLength:]
		if number == 1 && wireType == protowire.BytesType {
			value, fieldLength := protowire.ConsumeBytes(rest)
			if fieldLength < 0 {
				return nil, false
			}
			return value, true
		}
		fieldLength := protowire.ConsumeFieldValue(number, wireType, rest)
		if fieldLength < 0 {
			return nil, false
		}
		rest = rest[fieldLength:]
	}
	return nil, false
}

func extractLanzouURL(line string) string {
	match := hyperlinkRe.FindStringSubmatch(line)
	if len(match) != 2 {
		match = plainURLRe.FindStringSubmatch(line)
	}
	if len(match) != 2 {
		return ""
	}
	link := strings.TrimSpace(match[1])
	if !strings.Contains(strings.ToLower(link), "lanzou") {
		return ""
	}
	return link
}

func isClearlyInvalidTencentFontName(value string) bool {
	name := cleanFontName(value)
	if name == "" || name == "2" {
		return true
	}
	for _, r := range name {
		if unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul, unicode.Latin) {
			return false
		}
	}
	return true
}

func cleanDecodedLine(line string) string {
	var b strings.Builder
	for _, r := range line {
		switch {
		case r == utf8.RuneError:
			b.WriteRune(' ')
		case r == '\t':
			b.WriteRune(' ')
		case unicode.IsPrint(r):
			b.WriteRune(r)
		case r < 32:
			b.WriteRune(' ')
		}
	}
	return strings.TrimSpace(regexp.MustCompile(`\s+`).ReplaceAllString(b.String(), " "))
}

func isLikelyFontLine(line string) bool {
	if len([]rune(line)) > 120 {
		return false
	}
	lower := strings.ToLower(line)
	if strings.Contains(lower, "http") || strings.Contains(lower, "lanzou") || strings.Contains(line, "关于下载") {
		return false
	}
	if strings.Contains(line, "腾讯文档") || strings.Contains(line, "HYPERLINK") {
		return false
	}
	return true
}
