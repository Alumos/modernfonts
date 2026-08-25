package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestEmptyListHandlersReturnJSONArrays(t *testing.T) {
	rt, _ := newAuthTestRuntime(t)
	tests := []struct {
		name    string
		path    string
		key     string
		handler func(*gin.Context)
	}{
		{name: "document sources", path: "/api/admin/sources", key: "sources", handler: rt.handleListSources},
		{name: "admin fonts", path: "/api/admin/fonts", key: "fonts", handler: rt.handleListFonts},
		{name: "parse runs", path: "/api/admin/parse-runs", key: "runs", handler: rt.handleListParseRuns},
		{name: "users", path: "/api/admin/users", key: "users", handler: rt.handleListUsers},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, test.path, nil)
			rec := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(rec)
			ctx.Request = req

			test.handler(ctx)

			if rec.Code != http.StatusOK {
				t.Fatalf("status code = %d, body = %s", rec.Code, rec.Body.String())
			}
			var response map[string]json.RawMessage
			if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if string(response[test.key]) != "[]" {
				t.Fatalf("%s JSON = %s, want []", test.key, response[test.key])
			}
		})
	}
}
