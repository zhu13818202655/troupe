package openai_compat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestNormalizeBaseURL pins the base-URL canonicalisation. These cases mirror
// what operators actually paste out of vLLM / LMDeploy / llama.cpp logs and the
// provider settings form, so a regression here breaks self-hosted model
// onboarding in a way that surfaces as an opaque 404.
func TestNormalizeBaseURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty stays empty", "", ""},
		{"whitespace only stays empty", "   ", ""},
		{"bare host gets v1", "http://localhost:8000", "http://localhost:8000/v1"},
		{"trailing slash gets v1", "http://localhost:8000/", "http://localhost:8000/v1"},
		{"already canonical is untouched", "http://localhost:8000/v1", "http://localhost:8000/v1"},

		// Regression: the old suffix check produced "/v1/v1".
		{"canonical with trailing slash is not doubled", "http://localhost:8000/v1/", "http://localhost:8000/v1"},
		{"surrounding whitespace is trimmed", "  http://localhost:8000/v1  ", "http://localhost:8000/v1"},

		// A full endpoint path pasted straight from the API docs.
		{"chat completions path is trimmed back", "http://localhost:8000/v1/chat/completions", "http://localhost:8000/v1"},
		{"models path is trimmed back", "http://localhost:8000/v1/models", "http://localhost:8000/v1"},
		{"embeddings path is trimmed back", "http://localhost:8000/v1/embeddings", "http://localhost:8000/v1"},
		{"endpoint path without v1 still gets v1", "http://localhost:8000/chat/completions", "http://localhost:8000/v1"},

		// Hosted gateways keep their own prefix, which must survive verbatim.
		{"gateway prefix is preserved", "https://gateway.example.com/openai/v1", "https://gateway.example.com/openai/v1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizeBaseURL(tc.in); got != tc.want {
				t.Fatalf("NormalizeBaseURL(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestListModels_RequestsNormalizedPath proves the normalisation is actually
// wired into the request path rather than merely available: a trailing slash or
// a pasted "/models" suffix must still hit exactly one "/v1/models".
func TestListModels_RequestsNormalizedPath(t *testing.T) {
	tests := []struct {
		name string
		// suffix is appended to the httptest server URL, so it represents what
		// the operator typed after the host.
		suffix   string
		wantPath string
	}{
		{"bare host", "", "/v1/models"},
		{"trailing slash", "/", "/v1/models"},
		{"canonical", "/v1", "/v1/models"},
		{"canonical with trailing slash", "/v1/", "/v1/models"},
		{"pasted models endpoint", "/v1/models", "/v1/models"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var gotPath string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{
					"object": "list",
					"data": []map[string]any{
						{"id": "self-hosted-model", "object": "model"},
					},
				})
			}))
			defer srv.Close()

			baseURL := srv.URL + tc.suffix

			models, err := ListModels(context.Background(), "EMPTY", baseURL)
			if err != nil {
				t.Fatalf("ListModels(%q): %v", baseURL, err)
			}
			if gotPath != tc.wantPath {
				t.Fatalf("request path = %q, want %q", gotPath, tc.wantPath)
			}
			if len(models) != 1 || models[0] != "self-hosted-model" {
				t.Fatalf("models = %v, want [self-hosted-model]", models)
			}
		})
	}
}
