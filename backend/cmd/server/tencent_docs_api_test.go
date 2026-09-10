package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseTencentDocOpenAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Access-Token") != "token" || r.Header.Get("Client-Id") != "client" || r.Header.Get("Open-Id") != "open" {
			http.Error(w, `{"message":"missing auth"}`, http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/openapi/drive/v2/util/converter":
			if r.URL.Query().Get("type") != "2" || r.URL.Query().Get("value") != "DEncodedID" {
				t.Fatalf("converter query = %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"ret":0,"msg":"Succeed","data":{"fileID":"300000000$FileID"}}`))
		case "/openapi/drive/v2/files/300000000$FileID/metadata":
			_, _ = w.Write([]byte(`{"ret":0,"msg":"Succeed","data":{"title":"私有字体文档"}}`))
		case "/openapi/doc/v3/300000000$FileID":
			response := tencentDocsDocumentResponse{Document: &tencentDocsNode{
				Type: "Document",
				Children: []tencentDocsNode{
					{Type: "Paragraph", Children: []tencentDocsNode{{Type: "Text", Text: "测试字体 六字重"}}},
					{Type: "Paragraph", Children: []tencentDocsNode{{Type: "begin", Text: "HYPERLINK https://example.lanzou.com/font dkey code123"}}},
				},
			}, Version: 1}
			_ = json.NewEncoder(w).Encode(response)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	previousBaseURL := tencentDocsAPIBaseURL
	tencentDocsAPIBaseURL = server.URL
	t.Cleanup(func() { tencentDocsAPIBaseURL = previousBaseURL })

	result, err := parseTencentDocOpenAPI(context.Background(), "https://docs.qq.com/doc/DEncodedID", tencentDocsAuth{
		ClientID: "client", AccessToken: "token", OpenID: "open",
	})
	if err != nil {
		t.Fatalf("parse OpenAPI document: %v", err)
	}
	if result.Title != "私有字体文档" || len(result.Items) != 1 {
		t.Fatalf("result = %#v", result)
	}
	item := result.Items[0]
	if item.FontName != "测试字体 六字重" || item.DownloadURL != "https://example.lanzou.com/font" || item.AccessCode != "code123" {
		t.Fatalf("item = %#v", item)
	}
}

func TestParseTencentDocOpenAPIRedactsCredentialsFromErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"message":"denied"}`, http.StatusUnauthorized)
	}))
	defer server.Close()
	previousBaseURL := tencentDocsAPIBaseURL
	tencentDocsAPIBaseURL = server.URL
	t.Cleanup(func() { tencentDocsAPIBaseURL = previousBaseURL })

	_, err := parseTencentDocOpenAPI(context.Background(), "https://docs.qq.com/doc/DEncodedID", tencentDocsAuth{
		ClientID: "client-secret-value", AccessToken: "token-secret-value", OpenID: "open-secret-value",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	for _, secret := range []string{"client-secret-value", "token-secret-value", "open-secret-value"} {
		if contains := strings.Contains(err.Error(), secret); contains {
			t.Fatalf("error exposes credential: %v", err)
		}
	}
}
