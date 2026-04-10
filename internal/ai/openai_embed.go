package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"
)

// embedRequestWire is the JSON shape sent to the /embeddings endpoint.
type embedRequestWire struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// embedResponseWire mirrors the OpenAI embeddings response shape.
type embedResponseWire struct {
	Data  []embedDataWire `json:"data"`
	Model string          `json:"model"`
	Usage *usageWire      `json:"usage"`
}

type embedDataWire struct {
	Index     int       `json:"index"`
	Embedding []float64 `json:"embedding"`
}

// Embed computes vector embeddings for one or more input texts.
func (p *openAICompatProvider) Embed(ctx context.Context, req EmbedRequest) (*EmbedResponse, error) {
	if len(req.Input) == 0 {
		return nil, &Error{Kind: ErrBadRequest, Message: "embed: input must not be empty"}
	}
	if len(req.Input) > MaxEmbedInputs {
		return nil, &Error{
			Kind:    ErrBadRequest,
			Message: fmt.Sprintf("embed: input count %d exceeds maximum %d", len(req.Input), MaxEmbedInputs),
		}
	}
	for i, s := range req.Input {
		if s == "" {
			return nil, &Error{
				Kind:    ErrBadRequest,
				Message: fmt.Sprintf("embed: input[%d] must not be an empty string", i),
			}
		}
	}

	apiKey, authErr := p.resolveAPIKey()
	if authErr != nil {
		return nil, authErr
	}

	model := req.Model
	if model == "" {
		model = p.cfg.EmbeddingModelOrDefault()
	}

	wire := embedRequestWire{
		Model: model,
		Input: req.Input,
	}
	httpReq, err := p.buildJSONRequest(ctx, "/embeddings", wire, apiKey)
	if err != nil {
		return nil, err
	}

	p.logEmbedStart(p.cfg.BaseURL, model, len(req.Input))

	resp, respBody, start, execErr := p.executeRequest(httpReq, apiKey)
	if execErr != nil {
		return nil, execErr
	}
	defer func() { _ = resp.Body.Close() }()

	return p.parseEmbedResponse(resp, respBody, apiKey, start, len(req.Input))
}

// parseEmbedResponse handles Embed-specific post-validation processing.
func (p *openAICompatProvider) parseEmbedResponse(
	resp *http.Response, respBody []byte, apiKey string, start time.Time, expectedCount int,
) (*EmbedResponse, error) {
	if validErr := p.validateResponse(resp, respBody, apiKey, start); validErr != nil {
		return nil, validErr
	}

	out, parseErr := decodeEmbedResponse(respBody, apiKey, expectedCount)
	if parseErr != nil {
		parseErr.Status = resp.StatusCode
		p.logRequestFailure(string(parseErr.Kind), resp.StatusCode, time.Since(start), parseErr.Message)
		return nil, parseErr
	}

	p.logEmbedSuccess(resp.StatusCode, out.Model, time.Since(start), out.Usage, len(out.Embeddings))
	return out, nil
}

// decodeEmbedResponse parses a successful 2xx JSON body into an
// *EmbedResponse. Sorts by index to handle providers that return
// embeddings out of order. Returns a typed *Error on any decode failure.
func decodeEmbedResponse(respBody []byte, apiKey string, expectedCount int) (*EmbedResponse, *Error) {
	var parsed embedResponseWire
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, &Error{
			Kind:    ErrBadResponse,
			Message: fmt.Sprintf("decode embed response: %v: %s", err, redactKey(snippet(respBody), apiKey)),
			cause:   err,
		}
	}

	if len(parsed.Data) == 0 {
		return nil, &Error{Kind: ErrBadResponse, Message: "upstream returned no embeddings"}
	}

	if len(parsed.Data) != expectedCount {
		return nil, &Error{
			Kind:    ErrBadResponse,
			Message: fmt.Sprintf("upstream returned %d embeddings, expected %d", len(parsed.Data), expectedCount),
		}
	}

	// Sort by index to handle out-of-order responses.
	sort.Slice(parsed.Data, func(i, j int) bool {
		return parsed.Data[i].Index < parsed.Data[j].Index
	})

	embeddings := make([][]float64, len(parsed.Data))
	for i, d := range parsed.Data {
		if len(d.Embedding) == 0 {
			return nil, &Error{
				Kind:    ErrBadResponse,
				Message: fmt.Sprintf("upstream returned empty embedding at index %d", d.Index),
			}
		}
		embeddings[i] = d.Embedding
	}

	out := &EmbedResponse{
		Embeddings: embeddings,
		Model:      parsed.Model,
	}
	if parsed.Usage != nil {
		out.Usage = Usage{
			PromptTokens: parsed.Usage.PromptTokens,
			TotalTokens:  parsed.Usage.TotalTokens,
		}
	}
	return out, nil
}

// logEmbedStart emits a debug log when an Embed request begins.
func (p *openAICompatProvider) logEmbedStart(baseURL, model string, inputCount int) {
	p.logger.Debug("ai embed start",
		"base_url", baseURL,
		"model", model,
		"inputs", inputCount)
}

// logEmbedSuccess emits an info log on successful embed response.
func (p *openAICompatProvider) logEmbedSuccess(
	status int, model string, latency time.Duration, usage Usage, vectors int,
) {
	p.logger.Info("ai embed ok",
		"status", status,
		"model", model,
		"latency_ms", latency.Milliseconds(),
		"vectors", vectors,
		"prompt_tokens", usage.PromptTokens,
		"total_tokens", usage.TotalTokens)
}
