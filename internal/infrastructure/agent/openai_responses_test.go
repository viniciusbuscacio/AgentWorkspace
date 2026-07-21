package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

func asMap(t *testing.T, value any) map[string]any {
	t.Helper()
	typed, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("value %T is not a map", value)
	}
	return typed
}

func asSlice(t *testing.T, value any) []any {
	t.Helper()
	typed, ok := value.([]any)
	if !ok {
		t.Fatalf("value %T is not a slice", value)
	}
	return typed
}

func TestResponsesPayloadFromChat(t *testing.T) {
	temperature := float32(0.4)
	payload := chatCompletionRequest{
		Model: "gpt-5.5",
		Messages: []chatMessage{
			{Role: "system", Content: "You are aw."},
			{Role: "user", Content: "leia note.txt"},
			{Role: "assistant", Content: "Vou ler.", ToolCalls: []oaToolCall{func() oaToolCall {
				call := oaToolCall{ID: "call_1", Type: "function"}
				call.Function.Name = "read_file"
				call.Function.Arguments = `{"path":"note.txt"}`
				return call
			}()}},
			{Role: "tool", Name: "read_file", ToolCallID: "call_1", Content: `{"text":"hello"}`},
			{Role: "user", Content: []chatContentPart{
				{Type: "text", Text: "veja"},
				{Type: "image_url", ImageURL: &chatImageURLPart{URL: "data:image/png;base64,AA=="}},
			}},
		},
		Temperature: &temperature,
		MaxTokens:   128,
		Tools: []oaTool{{Type: "function", Function: oaToolFunction{
			Name:        "read_file",
			Description: "Read a file",
			Parameters:  map[string]any{"type": "object"},
		}}},
	}
	converted := responsesPayloadFromChat(payload)
	if converted["model"] != "gpt-5.5" || converted["instructions"] != "You are aw." {
		t.Fatalf("converted header = %+v", converted)
	}
	if converted["max_output_tokens"] != int32(128) {
		t.Errorf("max_output_tokens = %v", converted["max_output_tokens"])
	}
	tools := asSlice(t, converted["tools"])
	if len(tools) != 1 || asMap(t, tools[0])["name"] != "read_file" {
		t.Errorf("tools = %+v", tools)
	}
	input := asSlice(t, converted["input"])
	// user text, assistant text, function_call, function_call_output, user multimodal
	if len(input) != 5 {
		t.Fatalf("input = %+v, want 5 items", input)
	}
	if item := asMap(t, input[1]); item["type"] != "message" || item["role"] != "assistant" {
		t.Errorf("assistant item = %+v", item)
	}
	if item := asMap(t, input[2]); item["type"] != "function_call" || item["call_id"] != "call_1" || item["name"] != "read_file" {
		t.Errorf("function_call item = %+v", item)
	}
	if item := asMap(t, input[3]); item["type"] != "function_call_output" || item["call_id"] != "call_1" {
		t.Errorf("function_call_output item = %+v", item)
	}
	multimodal := asSlice(t, asMap(t, input[4])["content"])
	if len(multimodal) != 2 || asMap(t, multimodal[1])["type"] != "input_image" {
		t.Errorf("multimodal content = %+v", multimodal)
	}
	// Round-trips through JSON (the wire format).
	if _, err := json.Marshal(converted); err != nil {
		t.Fatalf("marshal converted payload: %v", err)
	}
}

func TestOpenAICompatibleModelStreamsViaResponsesEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var captured map[string]any
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if captured["stream"] != true {
			t.Fatalf("expected stream=true, got %+v", captured["stream"])
		}
		if _, hasMessages := captured["messages"]; hasMessages {
			t.Fatalf("responses payload must not carry chat messages: %+v", captured)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		events := []string{
			`data: {"type":"response.output_text.delta","delta":"Vou consultar. "}`,
			`data: {"type":"response.completed","response":{"model":"gpt-5.5","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Vou consultar."}]},{"type":"function_call","call_id":"call_9","name":"read_file","arguments":"{\"path\":\"note.txt\"}"}],"usage":{"input_tokens":9,"output_tokens":4,"total_tokens":13}}}`,
		}
		for _, event := range events {
			_, _ = w.Write([]byte(event + "\n\n"))
		}
	}))
	defer server.Close()

	var resolvedModel string
	llm, err := NewOpenAICompatibleModel(OpenAICompatibleConfig{
		ProviderID: "github-copilot",
		Model:      "gpt-5.5",
		APIKey:     "test-key",
		BaseURL:    server.URL,
		UseResponsesEndpoint: func(_ context.Context, modelName string) (bool, error) {
			resolvedModel = modelName
			return true, nil
		},
	})
	if err != nil {
		t.Fatalf("NewOpenAICompatibleModel() error = %v", err)
	}

	var partials []string
	var final *model.LLMResponse
	for response, err := range llm.GenerateContent(context.Background(), toolRequest(), true) {
		if err != nil {
			t.Fatalf("GenerateContent() error = %v", err)
		}
		if response.Partial {
			partials = append(partials, contentText(response.Content))
			continue
		}
		final = response
	}
	if resolvedModel != "demo" {
		t.Errorf("endpoint resolver received model %q, want the request's model", resolvedModel)
	}
	if strings.Join(partials, "") != "Vou consultar. " {
		t.Fatalf("partials = %q", strings.Join(partials, ""))
	}
	if final == nil || !final.TurnComplete {
		t.Fatalf("final response = %+v, want TurnComplete", final)
	}
	if len(final.Content.Parts) != 2 {
		t.Fatalf("final parts = %+v, want text and function call", final.Content.Parts)
	}
	call := final.Content.Parts[1].FunctionCall
	if call == nil || call.ID != "call_9" || call.Name != "read_file" || call.Args["path"] != "note.txt" {
		t.Fatalf("function call = %+v", call)
	}
	if final.UsageMetadata == nil || final.UsageMetadata.TotalTokenCount != 13 {
		t.Fatalf("usage = %+v, want total=13", final.UsageMetadata)
	}
	if final.FinishReason != genai.FinishReasonStop {
		// tool_calls maps like the chat path: genai has no tool-call finish
		// reason, finishReason() returns Stop for both.
		t.Logf("finish reason = %v", final.FinishReason)
	}
}

func TestStripUnsupportedResponsesParam(t *testing.T) {
	request := map[string]any{"model": "gpt-5.5", "input": []any{}, "temperature": 0.4}
	if !stripUnsupportedResponsesParam(request, "Unsupported parameter: 'temperature' is not supported with this model.") {
		t.Fatalf("expected temperature to be stripped")
	}
	if _, present := request["temperature"]; present {
		t.Errorf("temperature still present: %+v", request)
	}
	// Absent parameter, structural parameter and unrelated errors: no retry.
	if stripUnsupportedResponsesParam(request, "Unsupported parameter: 'temperature' is not supported with this model.") {
		t.Errorf("second strip of the same parameter must not signal a retry")
	}
	if stripUnsupportedResponsesParam(request, "Unsupported parameter: 'model'") {
		t.Errorf("structural parameters must never be stripped")
	}
	if stripUnsupportedResponsesParam(request, "some other error") {
		t.Errorf("unrelated errors must not signal a retry")
	}
}

func TestResponsesEndpointRetriesWithoutRejectedParameter(t *testing.T) {
	var requests []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var captured map[string]any
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		requests = append(requests, captured)
		if _, hasTemperature := captured["temperature"]; hasTemperature {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{"message": "Unsupported parameter: 'temperature' is not supported with this model."},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model":  "gpt-5.5",
			"status": "completed",
			"output": []map[string]any{
				{"type": "message", "role": "assistant", "content": []map[string]any{
					{"type": "output_text", "text": "ok"},
				}},
			},
		})
	}))
	defer server.Close()

	llm, err := NewOpenAICompatibleModel(OpenAICompatibleConfig{
		ProviderID:           "github-copilot",
		Model:                "gpt-5.5",
		APIKey:               "test-key",
		BaseURL:              server.URL,
		UseResponsesEndpoint: func(context.Context, string) (bool, error) { return true, nil },
	})
	if err != nil {
		t.Fatalf("NewOpenAICompatibleModel() error = %v", err)
	}
	temperature := float32(0.4)
	req := &model.LLMRequest{
		Model:    "gpt-5.5",
		Contents: []*genai.Content{genai.NewContentFromText("oi", genai.RoleUser)},
		Config:   &genai.GenerateContentConfig{Temperature: &temperature},
	}
	var final *model.LLMResponse
	for response, err := range llm.GenerateContent(context.Background(), req, false) {
		if err != nil {
			t.Fatalf("GenerateContent() error = %v", err)
		}
		final = response
	}
	if final == nil || contentText(final.Content) != "ok" {
		t.Fatalf("final = %+v", final)
	}
	if len(requests) != 2 {
		t.Fatalf("requests = %d, want 2 (rejected then retried)", len(requests))
	}
	if _, hasTemperature := requests[1]["temperature"]; hasTemperature {
		t.Errorf("retried request still carries temperature: %+v", requests[1])
	}
}

func TestOpenAICompatibleModelGeneratesViaResponsesEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model":  "gpt-5.5",
			"status": "completed",
			"output": []map[string]any{
				{"type": "reasoning"},
				{"type": "message", "role": "assistant", "content": []map[string]any{
					{"type": "output_text", "text": "olá!"},
				}},
			},
			"usage": map[string]any{"input_tokens": 3, "output_tokens": 2, "total_tokens": 5},
		})
	}))
	defer server.Close()

	llm, err := NewOpenAICompatibleModel(OpenAICompatibleConfig{
		ProviderID:           "github-copilot",
		Model:                "gpt-5.5",
		APIKey:               "test-key",
		BaseURL:              server.URL,
		UseResponsesEndpoint: func(context.Context, string) (bool, error) { return true, nil },
	})
	if err != nil {
		t.Fatalf("NewOpenAICompatibleModel() error = %v", err)
	}
	var final *model.LLMResponse
	for response, err := range llm.GenerateContent(context.Background(), &model.LLMRequest{
		Model:    "gpt-5.5",
		Contents: []*genai.Content{genai.NewContentFromText("oi", genai.RoleUser)},
	}, false) {
		if err != nil {
			t.Fatalf("GenerateContent() error = %v", err)
		}
		final = response
	}
	if final == nil || contentText(final.Content) != "olá!" {
		t.Fatalf("final = %+v", final)
	}
	if final.UsageMetadata == nil || final.UsageMetadata.TotalTokenCount != 5 {
		t.Fatalf("usage = %+v", final.UsageMetadata)
	}
}
