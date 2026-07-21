package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"net/http"
	"regexp"
	"strings"

	"aw/internal/domain"
	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

// OpenAI Responses API support. Newer models (e.g. GitHub Copilot's gpt-5.5)
// are only served through POST /responses and reject /chat/completions. The
// adapter keeps a single source of truth for history/tool translation — the
// chat-completions payload built by buildRequest — and converts that payload
// to the Responses wire format here, so both endpoints stay behaviorally
// identical. Which endpoint a request uses is decided per model by the
// provider (UseResponsesEndpoint), defaulting to /chat/completions.

// responsesPayloadFromChat converts a chat-completions payload into the
// Responses API shape: system messages become top-level instructions, tool
// messages become function_call_output items, assistant tool_calls become
// function_call items and multimodal parts map to input_text/input_image.
func responsesPayloadFromChat(payload chatCompletionRequest) map[string]any {
	request := map[string]any{
		"model":  payload.Model,
		"stream": payload.Stream,
	}
	var instructions []string
	input := make([]any, 0, len(payload.Messages))
	for _, msg := range payload.Messages {
		switch msg.Role {
		case "system":
			if text := strings.TrimSpace(messageContentText(msg.Content)); text != "" {
				instructions = append(instructions, text)
			}
		case "tool":
			input = append(input, map[string]any{
				"type":    "function_call_output",
				"call_id": msg.ToolCallID,
				"output":  messageContentText(msg.Content),
			})
		case "assistant":
			if text := strings.TrimSpace(messageContentText(msg.Content)); text != "" {
				input = append(input, map[string]any{
					"type": "message",
					"role": "assistant",
					"content": []any{
						map[string]any{"type": "output_text", "text": text},
					},
				})
			}
			for _, call := range msg.ToolCalls {
				input = append(input, map[string]any{
					"type":      "function_call",
					"call_id":   call.ID,
					"name":      call.Function.Name,
					"arguments": call.Function.Arguments,
				})
			}
		default:
			input = append(input, map[string]any{
				"type":    "message",
				"role":    "user",
				"content": responsesUserContent(msg.Content),
			})
		}
	}
	request["input"] = input
	if len(instructions) > 0 {
		request["instructions"] = strings.Join(instructions, "\n\n")
	}
	if len(payload.Tools) > 0 {
		tools := make([]any, 0, len(payload.Tools))
		for _, tool := range payload.Tools {
			tools = append(tools, map[string]any{
				"type":        "function",
				"name":        tool.Function.Name,
				"description": tool.Function.Description,
				"parameters":  tool.Function.Parameters,
			})
		}
		request["tools"] = tools
	}
	if payload.Temperature != nil {
		request["temperature"] = *payload.Temperature
	}
	if payload.TopP != nil {
		request["top_p"] = *payload.TopP
	}
	if payload.MaxTokens > 0 {
		request["max_output_tokens"] = payload.MaxTokens
	}
	return request
}

// responsesUserContent maps a chat message content (string or multimodal
// part array) to Responses user content parts.
func responsesUserContent(content any) []any {
	switch typed := content.(type) {
	case string:
		return []any{map[string]any{"type": "input_text", "text": typed}}
	case []chatContentPart:
		parts := make([]any, 0, len(typed))
		for _, part := range typed {
			switch {
			case part.Type == "text" && part.Text != "":
				parts = append(parts, map[string]any{"type": "input_text", "text": part.Text})
			case part.Type == "image_url" && part.ImageURL != nil:
				parts = append(parts, map[string]any{"type": "input_image", "image_url": part.ImageURL.URL})
			}
		}
		return parts
	default:
		return []any{}
	}
}

type responsesAPIResponse struct {
	ID                string `json:"id"`
	Model             string `json:"model"`
	Status            string `json:"status"`
	IncompleteDetails *struct {
		Reason string `json:"reason"`
	} `json:"incomplete_details"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
	Output []responsesOutputItem `json:"output"`
	Usage  *responsesUsage       `json:"usage"`
}

type responsesOutputItem struct {
	Type    string `json:"type"`
	Role    string `json:"role"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	CallID    string `json:"call_id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type responsesUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// responsesStreamEvent is a single Responses SSE data payload; Type
// discriminates ("response.output_text.delta", "response.completed", ...).
type responsesStreamEvent struct {
	Type     string                `json:"type"`
	Delta    string                `json:"delta"`
	Message  string                `json:"message"`
	Response *responsesAPIResponse `json:"response"`
}

func chatUsageFromResponses(usage *responsesUsage) *chatCompletionUsage {
	if usage == nil {
		return nil
	}
	return &chatCompletionUsage{
		PromptTokens:     usage.InputTokens,
		CompletionTokens: usage.OutputTokens,
		TotalTokens:      usage.TotalTokens,
	}
}

// responsesFinishReason maps the response status onto the chat-completions
// finish-reason vocabulary the rest of the adapter speaks.
func responsesFinishReason(resp *responsesAPIResponse) string {
	switch resp.Status {
	case "completed":
		for _, item := range resp.Output {
			if item.Type == "function_call" {
				return "tool_calls"
			}
		}
		return "stop"
	case "incomplete":
		if resp.IncompleteDetails != nil && resp.IncompleteDetails.Reason == "max_output_tokens" {
			return "length"
		}
		return "length"
	default:
		return ""
	}
}

// extractResponsesOutput flattens a Responses output list into the assistant
// text plus chat-style tool calls, so downstream handling is shared.
func extractResponsesOutput(resp *responsesAPIResponse) (string, []oaToolCall) {
	var text strings.Builder
	var toolCalls []oaToolCall
	for _, item := range resp.Output {
		switch item.Type {
		case "message":
			for _, part := range item.Content {
				if part.Type == "output_text" && part.Text != "" {
					text.WriteString(part.Text)
				}
			}
		case "function_call":
			call := oaToolCall{ID: item.CallID, Type: "function"}
			call.Function.Name = item.Name
			call.Function.Arguments = item.Arguments
			toolCalls = append(toolCalls, call)
		}
	}
	return text.String(), toolCalls
}

// finalResponseFromParts assembles the terminal LLMResponse shared by the
// streaming and non-streaming Responses paths, applying the same leaked
// tool-call sanitation as the chat-completions path.
func (m *OpenAICompatibleModel) finalResponseFromParts(ctx context.Context, requestJSON string, text string, toolCalls []oaToolCall, modelVersion string, usage *chatCompletionUsage, finish string) (*model.LLMResponse, error) {
	text = stripVendorToolMarkup(strings.TrimSpace(text))
	if len(toolCalls) == 0 {
		if cleaned, leaked := parseLeakedToolCalls(text); len(leaked) > 0 {
			text, toolCalls = cleaned, leaked
		}
	}
	if len(toolCalls) > 0 {
		parts, err := toolCallParts(text, toolCalls)
		if err != nil {
			return nil, err
		}
		m.observeTurn(ctx, requestJSON, text, toolCalls, modelVersion, usage, finish)
		return &model.LLMResponse{
			Content:       &genai.Content{Role: genai.RoleModel, Parts: parts},
			ModelVersion:  firstNonEmpty(modelVersion, m.model),
			TurnComplete:  true,
			FinishReason:  finishReason(finish),
			UsageMetadata: usageMetadataFromOpenAI(usage),
			CustomMetadata: map[string]any{
				"provider": m.providerID,
			},
		}, nil
	}
	if text == "" {
		return nil, fmt.Errorf("model response returned no text")
	}
	m.observeTurn(ctx, requestJSON, text, nil, modelVersion, usage, finish)
	return &model.LLMResponse{
		Content:       genai.NewContentFromText(text, genai.RoleModel),
		ModelVersion:  firstNonEmpty(modelVersion, m.model),
		TurnComplete:  true,
		FinishReason:  finishReason(finish),
		UsageMetadata: usageMetadataFromOpenAI(usage),
		CustomMetadata: map[string]any{
			"provider": m.providerID,
		},
	}, nil
}

// unsupportedParameterPattern matches OpenAI's "Unsupported parameter: 'x'"
// rejections. Which sampling knobs each model accepts varies (reasoning models
// reject temperature/top_p entirely), so instead of hardcoding per-model rules
// the adapter drops the named parameter and retries.
var unsupportedParameterPattern = regexp.MustCompile(`(?i)unsupported parameter: '([a-z0-9_.]+)'`)

// stripUnsupportedResponsesParam removes the parameter a 400 error names from
// the request, returning true when something was actually removed (i.e. a
// retry is worthwhile). Structural keys are never dropped.
func stripUnsupportedResponsesParam(request map[string]any, errMessage string) bool {
	match := unsupportedParameterPattern.FindStringSubmatch(errMessage)
	if len(match) != 2 {
		return false
	}
	param := match[1]
	switch param {
	case "model", "input", "stream":
		return false
	}
	if _, present := request[param]; !present {
		return false
	}
	delete(request, param)
	return true
}

// doResponsesRequest POSTs the request map to /responses, dropping parameters
// the model rejects (e.g. temperature on reasoning models) and retrying.
// Returns the response body and the request JSON that finally succeeded.
func (m *OpenAICompatibleModel) doResponsesRequest(ctx context.Context, request map[string]any, streaming bool) (*http.Response, string, error) {
	baseURL, err := m.resolveBaseURL(ctx)
	if err != nil {
		return nil, "", err
	}
	for attempt := 0; ; attempt++ {
		body, err := json.Marshal(request)
		if err != nil {
			return nil, "", err
		}
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/responses", bytes.NewReader(body))
		if err != nil {
			return nil, "", err
		}
		httpReq.Header.Set("Content-Type", "application/json")
		if streaming {
			httpReq.Header.Set("Accept", "text/event-stream")
		}
		if err := m.applyAuthHeaders(ctx, httpReq); err != nil {
			return nil, "", err
		}
		httpResp, err := m.client.Do(httpReq)
		if err != nil {
			return nil, "", err
		}
		if httpResp.StatusCode >= 200 && httpResp.StatusCode < 300 {
			return httpResp, string(body), nil
		}
		raw, _ := io.ReadAll(io.LimitReader(httpResp.Body, 1024*1024))
		_ = httpResp.Body.Close()
		message := responseErrorMessage(raw)
		if httpResp.StatusCode == http.StatusBadRequest && attempt < 4 && stripUnsupportedResponsesParam(request, message) {
			continue
		}
		return nil, "", domain.NewProviderHTTPError(httpResp.StatusCode, httpResp.Status, message)
	}
}

// generateResponses is the non-streaming POST /responses path.
func (m *OpenAICompatibleModel) generateResponses(ctx context.Context, req *model.LLMRequest) (*model.LLMResponse, error) {
	payload, err := m.buildRequest(req)
	if err != nil {
		return nil, err
	}
	httpResp, requestJSON, err := m.doResponsesRequest(ctx, responsesPayloadFromChat(payload), false)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(httpResp.Body, 4*1024*1024))
	if err != nil {
		return nil, err
	}
	var decoded responsesAPIResponse
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, fmt.Errorf("decode responses API response: %w", err)
	}
	if decoded.Error != nil && strings.TrimSpace(decoded.Error.Message) != "" {
		return nil, errors.New(decoded.Error.Message)
	}
	text, toolCalls := extractResponsesOutput(&decoded)
	return m.finalResponseFromParts(ctx, requestJSON, text, toolCalls, decoded.Model, chatUsageFromResponses(decoded.Usage), responsesFinishReason(&decoded))
}

// generateResponsesStream is the streaming POST /responses path. Text deltas
// are yielded as partials; the terminal response.completed event carries the
// authoritative full output (text, tool calls, usage) for the final event.
func (m *OpenAICompatibleModel) generateResponsesStream(ctx context.Context, req *model.LLMRequest) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		payload, err := m.buildRequest(req)
		if err != nil {
			yield(nil, err)
			return
		}
		payload.Stream = true
		httpResp, requestJSON, err := m.doResponsesRequest(ctx, responsesPayloadFromChat(payload), true)
		if err != nil {
			yield(nil, err)
			return
		}
		defer httpResp.Body.Close()

		var sanitizer streamSanitizer
		var final *responsesAPIResponse
		scanner := bufio.NewScanner(httpResp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || !strings.HasPrefix(line, "data:") {
				continue
			}
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "[DONE]" {
				break
			}
			var event responsesStreamEvent
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}
			switch event.Type {
			case "response.output_text.delta":
				if event.Delta == "" {
					continue
				}
				if safe := sanitizer.Push(event.Delta); safe != "" {
					partial := &model.LLMResponse{
						Content: genai.NewContentFromText(safe, genai.RoleModel),
						Partial: true,
					}
					if !yield(partial, nil) {
						return
					}
				}
			case "response.completed", "response.incomplete":
				if event.Response != nil {
					final = event.Response
				}
			case "response.failed":
				if event.Response != nil && event.Response.Error != nil && strings.TrimSpace(event.Response.Error.Message) != "" {
					yield(nil, errors.New(event.Response.Error.Message))
					return
				}
				yield(nil, fmt.Errorf("responses stream reported failure"))
				return
			case "error":
				if strings.TrimSpace(event.Message) != "" {
					yield(nil, errors.New(event.Message))
					return
				}
			}
		}
		if err := scanner.Err(); err != nil {
			yield(nil, err)
			return
		}
		if tail := sanitizer.Flush(); tail != "" {
			if !yield(&model.LLMResponse{
				Content: genai.NewContentFromText(tail, genai.RoleModel),
				Partial: true,
			}, nil) {
				return
			}
		}
		if final == nil {
			// Stream ended without a terminal event; retry non-streaming so the
			// turn still completes.
			resp, generateErr := m.generateResponses(ctx, req)
			if generateErr != nil {
				yield(nil, fmt.Errorf("responses stream ended without completion; fallback failed: %w", generateErr))
				return
			}
			yield(resp, nil)
			return
		}
		if final.Error != nil && strings.TrimSpace(final.Error.Message) != "" {
			yield(nil, errors.New(final.Error.Message))
			return
		}
		text, toolCalls := extractResponsesOutput(final)
		resp, err := m.finalResponseFromParts(ctx, requestJSON, text, toolCalls, final.Model, chatUsageFromResponses(final.Usage), responsesFinishReason(final))
		if err != nil {
			yield(nil, err)
			return
		}
		yield(resp, nil)
	}
}
