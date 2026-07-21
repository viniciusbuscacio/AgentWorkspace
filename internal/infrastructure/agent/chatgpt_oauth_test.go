package agent

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

func TestParseChatGPTCredentialExtractsAccountID(t *testing.T) {
	credential := testChatGPTCredential("access-token", "acct_123")
	parsed, err := parseChatGPTCredential(credential)
	if err != nil {
		t.Fatalf("parseChatGPTCredential() error = %v", err)
	}
	if parsed.AccessToken != "access-token" || parsed.AccountID != "acct_123" {
		t.Fatalf("parsed credential = %+v", parsed)
	}
}

func TestChatGPTOAuthModelStreamsResponsesEndpoint(t *testing.T) {
	var gotAuth string
	var gotAccount string
	var gotPath string
	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotAccount = r.Header.Get("ChatGPT-Account-ID")
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: response.output_text.delta\n" +
			"data: {\"type\":\"response.output_text.delta\",\"delta\":\"Olá\"}\n\n" +
			"event: response.output_text.delta\n" +
			"data: {\"type\":\"response.output_text.delta\",\"delta\":\" mundo\"}\n\n" +
			"event: response.completed\n" +
			"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_1\",\"model\":\"gpt-5.5\",\"usage\":{\"input_tokens\":3,\"output_tokens\":2,\"total_tokens\":5}}}\n\n"))
	}))
	defer server.Close()

	llm, err := NewChatGPTOAuthModel(ChatGPTOAuthConfig{
		ProviderID: "openai-codex",
		Model:      "gpt-5.5",
		Credential: testChatGPTCredential("access-token", "acct_123"),
		BaseURL:    server.URL,
	})
	if err != nil {
		t.Fatalf("NewChatGPTOAuthModel() error = %v", err)
	}
	req := &model.LLMRequest{
		Model: "gpt-5.5",
		Contents: []*genai.Content{
			genai.NewContentFromText("oi", genai.RoleUser),
		},
	}
	var final *model.LLMResponse
	var partials strings.Builder
	for response, err := range llm.GenerateContent(context.Background(), req, true) {
		if err != nil {
			t.Fatalf("GenerateContent() error = %v", err)
		}
		if response.Partial {
			partials.WriteString(contentText(response.Content))
			continue
		}
		final = response
	}
	if gotPath != "/responses" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotAuth != "Bearer access-token" || gotAccount != "acct_123" {
		t.Fatalf("auth headers = %q / %q", gotAuth, gotAccount)
	}
	if gotPayload["model"] != "gpt-5.5" || gotPayload["stream"] != true {
		t.Fatalf("payload = %+v", gotPayload)
	}
	if partials.String() != "Olá mundo" {
		t.Fatalf("partials = %q", partials.String())
	}
	if final == nil || contentText(final.Content) != "Olá mundo" {
		t.Fatalf("final response = %+v", final)
	}
	if final.UsageMetadata == nil || final.UsageMetadata.TotalTokenCount != 5 {
		t.Fatalf("usage = %+v", final.UsageMetadata)
	}
}

func TestChatGPTOAuthModelAdvertisesOnlyAWTool(t *testing.T) {
	var gotPayload chatGPTResponsesRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: response.output_text.delta\n" +
			"data: {\"type\":\"response.output_text.delta\",\"delta\":\"ok\"}\n\n" +
			"event: response.completed\n" +
			"data: {\"type\":\"response.completed\",\"response\":{\"model\":\"gpt-5.5\"}}\n\n"))
	}))
	defer server.Close()

	llm, err := NewChatGPTOAuthModel(ChatGPTOAuthConfig{Model: "gpt-5.5", Credential: testChatGPTCredential("access-token", "acct_123"), BaseURL: server.URL})
	if err != nil {
		t.Fatalf("NewChatGPTOAuthModel() error = %v", err)
	}
	for _, err := range llm.GenerateContent(context.Background(), awToolRequest(), false) {
		if err != nil {
			t.Fatalf("GenerateContent() error = %v", err)
		}
	}
	if len(gotPayload.Tools) != 1 || gotPayload.Tools[0].Name != "aw" || gotPayload.Tools[0].Type != "function" {
		t.Fatalf("tools payload = %+v, want only aw", gotPayload.Tools)
	}
}

func TestChatGPTOAuthModelParsesFunctionCall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: response.output_item.done\n" +
			"data: {\"type\":\"response.output_item.done\",\"item\":{\"type\":\"function_call\",\"call_id\":\"call_1\",\"name\":\"aw\",\"arguments\":\"{\\\"action\\\":\\\"memory.remember\\\",\\\"args\\\":\\\"{}\\\"}\"}}\n\n" +
			"event: response.completed\n" +
			"data: {\"type\":\"response.completed\",\"response\":{\"model\":\"gpt-5.5\"}}\n\n"))
	}))
	defer server.Close()

	llm, err := NewChatGPTOAuthModel(ChatGPTOAuthConfig{Model: "gpt-5.5", Credential: testChatGPTCredential("access-token", "acct_123"), BaseURL: server.URL})
	if err != nil {
		t.Fatalf("NewChatGPTOAuthModel() error = %v", err)
	}
	var final *model.LLMResponse
	for response, err := range llm.GenerateContent(context.Background(), awToolRequest(), false) {
		if err != nil {
			t.Fatalf("GenerateContent() error = %v", err)
		}
		final = response
	}
	if final == nil || len(final.Content.Parts) != 1 || final.Content.Parts[0].FunctionCall == nil {
		t.Fatalf("final parts = %+v, want function call", final)
	}
	call := final.Content.Parts[0].FunctionCall
	if call.ID != "call_1" || call.Name != "aw" || call.Args["action"] != "memory.remember" {
		t.Fatalf("function call = %+v", call)
	}
}

func TestChatGPTOAuthModelReplaysFunctionOutput(t *testing.T) {
	var gotPayload chatGPTResponsesRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: response.output_text.delta\n" +
			"data: {\"type\":\"response.output_text.delta\",\"delta\":\"done\"}\n\n" +
			"event: response.completed\n" +
			"data: {\"type\":\"response.completed\",\"response\":{\"model\":\"gpt-5.5\"}}\n\n"))
	}))
	defer server.Close()

	llm, err := NewChatGPTOAuthModel(ChatGPTOAuthConfig{Model: "gpt-5.5", Credential: testChatGPTCredential("access-token", "acct_123"), BaseURL: server.URL})
	if err != nil {
		t.Fatalf("NewChatGPTOAuthModel() error = %v", err)
	}
	for _, err := range llm.GenerateContent(context.Background(), awToolReplayRequest(), false) {
		if err != nil {
			t.Fatalf("GenerateContent() error = %v", err)
		}
	}
	var sawCall, sawOutput bool
	for _, item := range gotPayload.Input {
		if item.Type == "function_call" && item.CallID == "call_1" && item.Name == "aw" {
			sawCall = true
		}
		if item.Type == "function_call_output" && item.CallID == "call_1" && strings.Contains(item.Output, "saved") {
			sawOutput = true
		}
	}
	if !sawCall || !sawOutput {
		t.Fatalf("input = %+v, want function call and output", gotPayload.Input)
	}
}

func TestDefaultModelFactorySupportsOAuthBrowser(t *testing.T) {
	_, err := DefaultModelFactory(context.Background(), ModelConfig{
		ProviderID: "openai-codex",
		AuthType:   "oauth-browser",
		Model:      "gpt-5.5",
		Credential: testChatGPTCredential("access-token", "acct_123"),
	})
	if err != nil {
		t.Fatalf("DefaultModelFactory oauth-browser error = %v", err)
	}
}

func TestChatGPTOAuthModelRefreshesCredentialOnUnauthorized(t *testing.T) {
	var responseCalls int
	var refreshGrant string
	var refreshToken string
	var persisted string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/responses":
			responseCalls++
			if responseCalls == 1 {
				http.Error(w, "expired", http.StatusUnauthorized)
				return
			}
			if got := r.Header.Get("Authorization"); got != "Bearer fresh-token" {
				t.Fatalf("Authorization after refresh = %q", got)
			}
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("event: response.output_text.delta\n" +
				"data: {\"type\":\"response.output_text.delta\",\"delta\":\"ok\"}\n\n" +
				"event: response.completed\n" +
				"data: {\"type\":\"response.completed\",\"response\":{\"model\":\"gpt-5.5\"}}\n\n"))
		case "/token":
			if err := r.ParseForm(); err != nil {
				t.Fatalf("ParseForm() error = %v", err)
			}
			refreshGrant = r.Form.Get("grant_type")
			refreshToken = r.Form.Get("refresh_token")
			_, _ = w.Write([]byte(`{"access_token":"fresh-token","refresh_token":"fresh-refresh","id_token":"id"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	llm, err := NewChatGPTOAuthModel(ChatGPTOAuthConfig{
		ProviderID: "openai-codex",
		Model:      "gpt-5.5",
		Credential: testChatGPTCredential("expired-token", "acct_123"),
		BaseURL:    server.URL,
		CredentialUpdater: func(next string) error {
			persisted = next
			return nil
		},
	})
	if err != nil {
		t.Fatalf("NewChatGPTOAuthModel() error = %v", err)
	}
	llm.tokenURL = server.URL + "/token"
	req := &model.LLMRequest{Contents: []*genai.Content{genai.NewContentFromText("oi", genai.RoleUser)}}
	for _, err := range llm.GenerateContent(context.Background(), req, true) {
		if err != nil {
			t.Fatalf("GenerateContent() error = %v", err)
		}
	}
	if responseCalls != 2 {
		t.Fatalf("responseCalls = %d, want first 401 plus retry", responseCalls)
	}
	if refreshGrant != "refresh_token" || refreshToken != "refresh-token" {
		t.Fatalf("refresh form = grant %q token %q", refreshGrant, refreshToken)
	}
	if !strings.Contains(persisted, "fresh-token") || !strings.Contains(persisted, "fresh-refresh") {
		t.Fatalf("persisted credential was not refreshed: %q", persisted)
	}
}

func awToolRequest() *model.LLMRequest {
	return &model.LLMRequest{
		Model:    "gpt-5.5",
		Contents: []*genai.Content{genai.NewContentFromText("lembre meu nome", genai.RoleUser)},
		Config: &genai.GenerateContentConfig{
			Tools: []*genai.Tool{{
				FunctionDeclarations: []*genai.FunctionDeclaration{{
					Name:        "aw",
					Description: "Single Agent Workspace tool dispatcher",
					ParametersJsonSchema: map[string]any{
						"type":     "object",
						"required": []string{"action"},
						"properties": map[string]any{
							"action": map[string]any{"type": "string"},
							"args":   map[string]any{"type": "string"},
						},
					},
				}},
			}},
		},
		Tools: map[string]any{"aw": struct{}{}},
	}
}

func awToolReplayRequest() *model.LLMRequest {
	req := awToolRequest()
	req.Contents = []*genai.Content{
		genai.NewContentFromText("lembre meu nome", genai.RoleUser),
		{Role: genai.RoleModel, Parts: []*genai.Part{{FunctionCall: &genai.FunctionCall{ID: "call_1", Name: "aw", Args: map[string]any{"action": "memory.remember", "args": "{}"}}}}},
		{Role: genai.RoleUser, Parts: []*genai.Part{{FunctionResponse: &genai.FunctionResponse{ID: "call_1", Name: "aw", Response: map[string]any{"result": "saved"}}}}},
	}
	return req
}

func testChatGPTCredential(accessToken string, accountID string) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	payload, _ := json.Marshal(map[string]any{"chatgpt_account_id": accountID})
	idToken := header + "." + base64.RawURLEncoding.EncodeToString(payload) + ".sig"
	body, _ := json.Marshal(map[string]any{
		"auth_mode": "chatgpt",
		"tokens": map[string]string{
			"access_token":  accessToken,
			"refresh_token": "refresh-token",
			"id_token":      idToken,
		},
	})
	return string(body)
}
