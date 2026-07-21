package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aw/internal/domain"
	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

func TestOpenAICompatibleModelStreamsToolCallDeltas(t *testing.T) {
	var captured chatCompletionRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if !captured.Stream {
			t.Fatalf("expected stream=true")
		}
		if len(captured.Tools) != 1 || captured.Tools[0].Function.Name != "read_file" {
			t.Fatalf("captured.Tools = %+v, want read_file", captured.Tools)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		chunks := []string{
			`data: {"model":"demo-stream","choices":[{"delta":{"content":"Vou consultar. "}}]}`,
			`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"read_file","arguments":"{\"pa"}}]}}]}`,
			`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"th\":\"note"}}]}}]}`,
			`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":".txt\"}"}}]},"finish_reason":"tool_calls"}]}`,
			`data: {"choices":[],"usage":{"prompt_tokens":9,"completion_tokens":4,"total_tokens":13}}`,
			`data: [DONE]`,
		}
		for _, chunk := range chunks {
			_, _ = w.Write([]byte(chunk + "\n\n"))
		}
	}))
	defer server.Close()

	llm, err := NewOpenAICompatibleModel(OpenAICompatibleConfig{
		ProviderID: "custom-openai",
		Model:      "demo",
		APIKey:     "test-key",
		BaseURL:    server.URL,
	})
	if err != nil {
		t.Fatalf("NewOpenAICompatibleModel() error = %v", err)
	}
	var observed []domain.LLMTurn
	llm.SetTurnObserver(func(_ context.Context, turn domain.LLMTurn) {
		observed = append(observed, turn)
	})

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
	if strings.Join(partials, "") != "Vou consultar. " {
		t.Fatalf("partials = %q", strings.Join(partials, ""))
	}
	if final == nil || !final.TurnComplete {
		t.Fatalf("final response = %+v, want TurnComplete", final)
	}
	if len(final.Content.Parts) != 2 {
		t.Fatalf("final parts = %+v, want text and function call", final.Content.Parts)
	}
	if final.Content.Parts[0].Text != "Vou consultar." {
		t.Fatalf("final text = %q", final.Content.Parts[0].Text)
	}
	call := final.Content.Parts[1].FunctionCall
	if call == nil || call.ID != "call_1" || call.Name != "read_file" || call.Args["path"] != "note.txt" {
		t.Fatalf("function call = %+v", call)
	}
	if final.UsageMetadata == nil || final.UsageMetadata.TotalTokenCount != 13 {
		t.Fatalf("usage = %+v, want total=13", final.UsageMetadata)
	}
	if len(observed) != 1 {
		t.Fatalf("observed turns = %d, want 1", len(observed))
	}
	if observed[0].ResponseText != "Vou consultar." ||
		!strings.Contains(observed[0].ToolCallsJSON, `"arguments":"{\"path\":\"note.txt\"}"`) ||
		observed[0].FinishReason != "tool_calls" {
		t.Fatalf("observed turn = %+v", observed[0])
	}
}

func TestOpenAICompatibleModelFallsBackWhenStreamedToolCallArgumentsAreInvalid(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var captured chatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if calls == 1 {
			if !captured.Stream {
				t.Fatalf("first request stream = false, want true")
			}
			w.Header().Set("Content-Type", "text/event-stream")
			chunks := []string{
				`data: {"model":"demo-stream","choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_bad","type":"function","function":{"name":"read_file","arguments":"{\"path\":"}}]},"finish_reason":"tool_calls"}]}`,
				`data: [DONE]`,
			}
			for _, chunk := range chunks {
				_, _ = w.Write([]byte(chunk + "\n\n"))
			}
			return
		}
		if captured.Stream {
			t.Fatalf("fallback request stream = true, want false")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"demo","choices":[{"message":{"role":"assistant","content":"fallback ok"},"finish_reason":"stop"}]}`))
	}))
	defer server.Close()

	llm, err := NewOpenAICompatibleModel(OpenAICompatibleConfig{
		ProviderID: "custom-openai",
		Model:      "demo",
		APIKey:     "test-key",
		BaseURL:    server.URL,
	})
	if err != nil {
		t.Fatalf("NewOpenAICompatibleModel() error = %v", err)
	}
	var observed []domain.LLMTurn
	llm.SetTurnObserver(func(_ context.Context, turn domain.LLMTurn) {
		observed = append(observed, turn)
	})

	var final *model.LLMResponse
	for response, err := range llm.GenerateContent(context.Background(), toolRequest(), true) {
		if err != nil {
			t.Fatalf("GenerateContent() error = %v", err)
		}
		final = response
	}
	if calls != 2 {
		t.Fatalf("requests = %d, want streaming attempt plus fallback", calls)
	}
	if final == nil || contentText(final.Content) != "fallback ok" || !final.TurnComplete {
		t.Fatalf("final = %+v", final)
	}
	if len(observed) != 1 || observed[0].ResponseText != "fallback ok" || observed[0].ToolCallsJSON != "[]" {
		t.Fatalf("observed = %+v", observed)
	}
}

func toolRequest() *model.LLMRequest {
	return &model.LLMRequest{
		Model:    "demo",
		Contents: []*genai.Content{genai.NewContentFromText("leia note.txt", genai.RoleUser)},
		Config: &genai.GenerateContentConfig{
			Tools: []*genai.Tool{{
				FunctionDeclarations: []*genai.FunctionDeclaration{{
					Name:        "read_file",
					Description: "Read a file",
					ParametersJsonSchema: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"path": map[string]any{"type": "string"},
						},
					},
				}},
			}},
		},
		Tools: map[string]any{"read_file": struct{}{}},
	}
}

// Pins the 2026-06-12 leak: deepseek-r1 emitted its tool-call wire format in
// the TEXT channel; the call never executed and the user saw the raw markers.
func TestStripVendorToolMarkup(t *testing.T) {
	leak := "Que tal \"Violet\"? Um instante!\nBanay\n```\nfunction<｜tool▁sep｜>aw\n{\"action\": \"app.theme.set\", \"args\": \"{\\\"theme\\\": \\\"violet\\\"}\"}\n```<｜tool▁call▁end｜><｜tool▁calls▁end｜>  \n\n(Executando a mudança...)\n\nPronto!"
	got := stripVendorToolMarkup(leak)
	for _, forbidden := range []string{"<｜tool▁sep｜>", "<｜tool▁call▁end｜>", "<｜tool▁calls▁end｜>", "app.theme.set"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("markup survived: %q in %q", forbidden, got)
		}
	}
	// The legitimate prose around the leak survives.
	for _, keep := range []string{"Que tal \"Violet\"?", "Pronto!"} {
		if !strings.Contains(got, keep) {
			t.Fatalf("legitimate prose lost: %q missing from %q", keep, got)
		}
	}
	// Clean text passes through untouched.
	if clean := "Tudo certo, tema aplicado."; stripVendorToolMarkup(clean) != clean {
		t.Fatal("clean text must pass through unchanged")
	}
}

// The live-stream variant of the leak: markers arrive SPLIT across deltas,
// and the user must never see them even transiently (they did on
// 2026-06-12: the markup streamed in, then "vanished" on reload).
func TestStreamSanitizerSuppressesSplitMarkers(t *testing.T) {
	leak := "Que tal \"Violet\"? Um instante!\nfunction<｜tool▁sep｜>aw\n{\"action\": \"app.theme.set\"}\n<｜tool▁call▁end｜><｜tool▁calls▁end｜>\nPronto! Tema aplicado."
	for _, size := range []int{1, 3, 7, len(leak)} {
		var s streamSanitizer
		var out strings.Builder
		for i := 0; i < len(leak); i += size {
			end := i + size
			if end > len(leak) {
				end = len(leak)
			}
			out.WriteString(s.Push(leak[i:end]))
		}
		out.WriteString(s.Flush())
		got := out.String()
		for _, forbidden := range []string{"<｜tool", "app.theme.set", "function<"} {
			if strings.Contains(got, forbidden) {
				t.Fatalf("chunk size %d: leaked %q in %q", size, forbidden, got)
			}
		}
		for _, keep := range []string{"Que tal \"Violet\"?", "Pronto! Tema aplicado."} {
			if !strings.Contains(got, keep) {
				t.Fatalf("chunk size %d: lost prose %q in %q", size, keep, got)
			}
		}
	}

	// Clean text streams through byte-identical, including lone < and ｜.
	clean := "a < b e c ｜ d — texto normal."
	var s streamSanitizer
	var out strings.Builder
	for _, r := range clean {
		out.WriteString(s.Push(string(r)))
	}
	out.WriteString(s.Flush())
	if out.String() != clean {
		t.Fatalf("clean text mutated: %q != %q", out.String(), clean)
	}
}

// Recover tool calls a model leaked as TEXT (DeepSeek-R1, 2026-06-12): the
// browser.close_tab never ran because the call arrived as a JSON array in the
// content channel. parseLeakedToolCalls must execute it AND clean the display.
func TestParseLeakedToolCalls(t *testing.T) {
	// arguments as an OBJECT (the transcript shape).
	text := "Vou fechar as abas.\n[{\"name\": \"aw\", \"arguments\": {\"action\": \"browser.close_tab\", \"args\": \"{\\\"title\\\": \\\"device activation\\\"}\"}}]"
	cleaned, calls := parseLeakedToolCalls(text)
	if len(calls) != 1 {
		t.Fatalf("expected 1 recovered call, got %d", len(calls))
	}
	if calls[0].Function.Name != "aw" {
		t.Fatalf("name = %q", calls[0].Function.Name)
	}
	if !strings.Contains(calls[0].Function.Arguments, "browser.close_tab") {
		t.Fatalf("arguments lost: %q", calls[0].Function.Arguments)
	}
	if strings.Contains(cleaned, "browser.close_tab") || strings.Contains(cleaned, "[{") {
		t.Fatalf("leaked JSON still visible: %q", cleaned)
	}
	if !strings.Contains(cleaned, "Vou fechar as abas.") {
		t.Fatalf("prose lost: %q", cleaned)
	}

	// arguments as a JSON STRING (the OpenAI-native shape leaked into text).
	text2 := "[{\"name\": \"aw\", \"arguments\": \"{\\\"action\\\": \\\"browser.status\\\"}\"}]"
	_, calls2 := parseLeakedToolCalls(text2)
	if len(calls2) != 1 || !strings.Contains(calls2[0].Function.Arguments, "browser.status") {
		t.Fatalf("string-arguments form not recovered: %+v", calls2)
	}

	// Ordinary prose and unrelated JSON are NOT treated as tool calls.
	for _, plain := range []string{
		"Aqui está a lista: [1, 2, 3].",
		"Exemplo: [{\"id\": 1, \"label\": \"x\"}]",
		"Sem nada de especial aqui.",
	} {
		if c, recovered := parseLeakedToolCalls(plain); recovered != nil || c != plain {
			t.Fatalf("false positive on %q: %+v", plain, recovered)
		}
	}
}
