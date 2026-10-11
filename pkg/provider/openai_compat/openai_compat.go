package openai_compat

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"iter"
	"log/slog"
	"net/http"
	"strings"

	"github.com/sashabaranov/go-openai"
	openai_provider "github.com/zhu13818202655/troupe/pkg/provider/openai"
	"google.golang.org/adk/model"
)

// Provider implements model.LLM for OpenAI Compatible endpoints.
type Provider struct {
	*openai_provider.Provider
}

// NormalizeBaseURL canonicalizes a user-supplied OpenAI-compatible base URL to
// the ".../v1" form the go-openai client expects.
//
// It is defensive on purpose: the previous suffix-only check turned
// "http://host:8000/v1/" into "http://host:8000/v1/v1" (a 404 that is very hard
// to diagnose) and appended "/v1" to full endpoint paths. Both are common when
// operators copy a URL straight out of vLLM / LMDeploy / llama.cpp output.
func NormalizeBaseURL(raw string) string {
	baseURL := strings.TrimSpace(raw)
	if baseURL == "" {
		return ""
	}
	// Trailing slashes first: "http://h:8000/v1/" must not become "/v1/v1".
	baseURL = strings.TrimRight(baseURL, "/")
	// Already canonical.
	if strings.HasSuffix(baseURL, "/v1") {
		return baseURL
	}
	// Someone pasted a full endpoint path — strip back to the API root.
	for _, suffix := range []string{"/chat/completions", "/models", "/embeddings"} {
		if strings.HasSuffix(baseURL, suffix) {
			baseURL = baseURL[:len(baseURL)-len(suffix)]
			break
		}
	}
	baseURL = strings.TrimRight(baseURL, "/")
	if strings.HasSuffix(baseURL, "/v1") {
		return baseURL
	}
	return baseURL + "/v1"
}

// NewProvider creates a new OpenAI Compatible provider.
//
// maxTokens caps the per-request `max_completion_tokens`. Use 0 or a negative
// value to omit the field entirely. Self-hosted stacks (vLLM, LMDeploy,
// llama.cpp) reject requests whose max_completion_tokens exceeds their
// max_model_len with HTTP 400, so a fixed 64000 is unusable for most local
// models; see the max_output_tokens provider key in pkg/provider/factory.go.
func NewProvider(apiKey, baseURL, modelName string, maxTokens int, debug bool) model.LLM {
	config := openai.DefaultConfig(apiKey)

	// Ensure baseURL has /v1 suffix
	baseURL = NormalizeBaseURL(baseURL)
	if baseURL != "" {
		config.BaseURL = baseURL
	}

	if debug {
		config.HTTPClient = &http.Client{
			Transport: &debugHTTPTransport{base: http.DefaultTransport},
		}
	}

	client := openai.NewClientWithConfig(config)

	return &Provider{
		Provider: openai_provider.NewProviderWithMaxTokens(client, modelName, true, maxTokens),
	}
}

// Name implements model.LLM.
func (p *Provider) Name() string {
	return p.Provider.Name()
}

// GenerateContent implements model.LLM.
func (p *Provider) GenerateContent(ctx context.Context, req *model.LLMRequest, streaming bool) iter.Seq2[*model.LLMResponse, error] {
	return p.Provider.GenerateContent(ctx, req, streaming)
}

// ListModels returns available models from an OpenAI compatible endpoint.
func ListModels(ctx context.Context, apiKey, baseURL string) ([]string, error) {
	config := openai.DefaultConfig(apiKey)

	baseURL = NormalizeBaseURL(baseURL)
	if baseURL != "" {
		config.BaseURL = baseURL
	}

	client := openai.NewClientWithConfig(config)

	resp, err := client.ListModels(ctx)
	if err != nil {
		return nil, err
	}

	var models []string
	for _, m := range resp.Models {
		models = append(models, m.ID)
	}
	return models, nil
}

// GetRequiredFields returns the required configuration fields for this provider.
func GetRequiredFields() []string {
	return []string{"api_key", "base_url"}
}

// debugHTTPTransport wraps an http.RoundTripper to log request and response
// details when debug mode is enabled. It captures:
//   - Request URL and method
//   - Request body (for POST requests)
//   - Response status code
//   - Response body (for non-2xx responses)
type debugHTTPTransport struct {
	base http.RoundTripper
}

func (t *debugHTTPTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	slog.Debug("http request", "method", req.Method, "url", req.URL.String())

	// Log request headers (redact Authorization)
	for key, vals := range req.Header {
		if strings.EqualFold(key, "Authorization") {
			slog.Debug("http request header", "header", key, "value", "[REDACTED]")
		} else {
			slog.Debug("http request header", "header", key, "value", strings.Join(vals, ", "))
		}
	}

	// Capture request body
	if req.Body != nil {
		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, fmt.Errorf("debug transport: failed to read request body: %w", err)
		}
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		slog.Debug("http request body", "bytes", len(bodyBytes), "body", string(bodyBytes))
	}

	// Perform the actual request
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		slog.Debug("http transport error", "error", err)
		return resp, err
	}

	slog.Debug("http response", "statusCode", resp.StatusCode, "status", resp.Status)

	// For non-2xx responses, capture the response body
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			slog.Debug("failed to read error response body", "error", readErr)
		} else {
			resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			slog.Debug("http error response body", "body", string(bodyBytes))
		}
	}

	return resp, nil
}
