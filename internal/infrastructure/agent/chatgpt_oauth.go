package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"aw/internal/domain"
	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

const defaultChatGPTCodexBaseURL = "https://chatgpt.com/backend-api/codex"

type ChatGPTOAuthConfig struct {
	ProviderID        string
	Model             string
	Credential        string
	BaseURL           string
	HTTPClient        *http.Client
	CredentialUpdater func(string) error
}

type ChatGPTOAuthModel struct {
	providerID        string
	model             string
	credential        chatGPTCredential
	baseURL           string
	client            *http.Client
	observer          func(context.Context, domain.LLMTurn)
	tokenURL          string
	refreshMu         sync.Mutex
	credentialUpdater func(string) error
}

type chatGPTCredential struct {
	AccessToken       string
	RefreshToken      string
	IDToken           string
	AccountID         string
	FedRAMP           bool
	RawCredentialJSON string
}

type chatGPTCredentialEnvelope struct {
	AuthMode string `json:"auth_mode"`
	Tokens   struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		IDToken      string `json:"id_token"`
	} `json:"tokens"`
}

type chatGPTIDClaims struct {
	ChatGPTAccountID        string `json:"chatgpt_account_id"`
	ChatGPTAccountIsFedRAMP bool   `json:"chatgpt_account_is_fedramp"`
}

type chatGPTResponsesRequest struct {
	Model             string           `json:"model"`
	Instructions      string           `json:"instructions,omitempty"`
	Input             []chatGPTItem    `json:"input"`
	Tools             []chatGPTTool    `json:"tools,omitempty"`
	ToolChoice        string           `json:"tool_choice"`
	ParallelToolCalls bool             `json:"parallel_tool_calls"`
	Store             bool             `json:"store"`
	Stream            bool             `json:"stream"`
	Include           []string         `json:"include"`
	Text              *chatGPTTextOpts `json:"text,omitempty"`
}

type chatGPTTextOpts struct {
	Verbosity string `json:"verbosity,omitempty"`
}

type chatGPTTool struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

type chatGPTItem struct {
	Type      string           `json:"type"`
	Role      string           `json:"role,omitempty"`
	Content   []chatGPTContent `json:"content,omitempty"`
	CallID    string           `json:"call_id,omitempty"`
	Name      string           `json:"name,omitempty"`
	Arguments string           `json:"arguments,omitempty"`
	Output    string           `json:"output,omitempty"`
}

type chatGPTContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type chatGPTTokenUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

func NewChatGPTOAuthModel(cfg ChatGPTOAuthConfig) (*ChatGPTOAuthModel, error) {
	if strings.TrimSpace(cfg.Model) == "" {
		return nil, fmt.Errorf("model is required")
	}
	credential, err := parseChatGPTCredential(cfg.Credential)
	if err != nil {
		return nil, err
	}
	client := cfg.HTTPClient
	if client == nil {
		client = defaultOpenAIHTTPClient()
	}
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = defaultChatGPTCodexBaseURL
	}
	return &ChatGPTOAuthModel{
		providerID:        strings.TrimSpace(cfg.ProviderID),
		model:             strings.TrimSpace(cfg.Model),
		credential:        credential,
		baseURL:           baseURL,
		client:            client,
		tokenURL:          "https://auth.openai.com/oauth/token",
		credentialUpdater: cfg.CredentialUpdater,
	}, nil
}

func parseChatGPTCredential(raw string) (chatGPTCredential, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return chatGPTCredential{}, fmt.Errorf("OAuth credential is required")
	}
	var envelope chatGPTCredentialEnvelope
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		return chatGPTCredential{}, fmt.Errorf("decode OAuth credential: %w", err)
	}
	credential := chatGPTCredential{
		AccessToken:       strings.TrimSpace(envelope.Tokens.AccessToken),
		RefreshToken:      strings.TrimSpace(envelope.Tokens.RefreshToken),
		IDToken:           strings.TrimSpace(envelope.Tokens.IDToken),
		RawCredentialJSON: raw,
	}
	if credential.AccessToken == "" {
		return chatGPTCredential{}, fmt.Errorf("OAuth credential does not include an access token")
	}
	if credential.IDToken != "" {
		claims, err := parseChatGPTIDClaims(credential.IDToken)
		if err == nil {
			credential.AccountID = strings.TrimSpace(claims.ChatGPTAccountID)
			credential.FedRAMP = claims.ChatGPTAccountIsFedRAMP
		}
	}
	return credential, nil
}

func buildChatGPTCredential(token map[string]any) (string, error) {
	if !nonEmptyTokenString(token, "access_token") || !nonEmptyTokenString(token, "refresh_token") {
		return "", fmt.Errorf("OAuth token refresh did not include access and refresh tokens")
	}
	payload := map[string]any{
		"auth_mode":    "chatgpt",
		"tokens":       token,
		"last_refresh": time.Now().UTC().Format(time.RFC3339),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("could not encode OAuth credential")
	}
	return string(data), nil
}

func nonEmptyTokenString(m map[string]any, key string) bool {
	value, ok := m[key]
	if !ok || value == nil {
		return false
	}
	text, ok := value.(string)
	return ok && strings.TrimSpace(text) != ""
}

func parseChatGPTIDClaims(jwt string) (chatGPTIDClaims, error) {
	parts := strings.Split(jwt, ".")
	if len(parts) < 2 {
		return chatGPTIDClaims{}, fmt.Errorf("invalid id token")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return chatGPTIDClaims{}, err
	}
	var claims chatGPTIDClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return chatGPTIDClaims{}, err
	}
	return claims, nil
}

func (m *ChatGPTOAuthModel) Name() string { return m.model }

func (m *ChatGPTOAuthModel) SetTurnObserver(observer func(context.Context, domain.LLMTurn)) {
	m.observer = observer
}

func (m *ChatGPTOAuthModel) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return m.generateStream(ctx, req, stream)
}

func (m *ChatGPTOAuthModel) generateStream(ctx context.Context, req *model.LLMRequest, partials bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		payload, err := m.buildRequest(req)
		if err != nil {
			yield(nil, err)
			return
		}
		body, err := json.Marshal(payload)
		if err != nil {
			yield(nil, err)
			return
		}
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, m.baseURL+"/responses", bytes.NewReader(body))
		if err != nil {
			yield(nil, err)
			return
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Accept", "text/event-stream")
		httpReq.Header.Set("Authorization", "Bearer "+m.credential.AccessToken)
		if m.credential.AccountID != "" {
			httpReq.Header.Set("ChatGPT-Account-ID", m.credential.AccountID)
		}
		if m.credential.FedRAMP {
			httpReq.Header.Set("X-OpenAI-Fedramp", "true")
		}

		httpResp, err := m.client.Do(httpReq)
		if err != nil {
			yield(nil, err)
			return
		}
		if httpResp.StatusCode == http.StatusUnauthorized && m.credential.RefreshToken != "" {
			_ = httpResp.Body.Close()
			if err := m.refreshCredential(ctx); err != nil {
				yield(nil, fmt.Errorf("OAuth credential expired and refresh failed; please sign in again"))
				return
			}
			httpReq, err = http.NewRequestWithContext(ctx, http.MethodPost, m.baseURL+"/responses", bytes.NewReader(body))
			if err != nil {
				yield(nil, err)
				return
			}
			httpReq.Header.Set("Content-Type", "application/json")
			httpReq.Header.Set("Accept", "text/event-stream")
			m.applyAuthHeaders(httpReq)
			httpResp, err = m.client.Do(httpReq)
			if err != nil {
				yield(nil, err)
				return
			}
		}
		defer httpResp.Body.Close()
		if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
			raw, _ := io.ReadAll(io.LimitReader(httpResp.Body, 1024*1024))
			yield(nil, domain.NewProviderHTTPError(httpResp.StatusCode, httpResp.Status, responseErrorMessage(raw)))
			return
		}

		text, toolCalls, modelVersion, usage, finish, err := m.readResponsesSSE(httpResp.Body, partials, yield)
		if err != nil {
			yield(nil, err)
			return
		}
		text = strings.TrimSpace(text)
		if len(toolCalls) > 0 {
			parts, err := toolCallParts(text, toolCalls)
			if err != nil {
				yield(nil, err)
				return
			}
			m.observeTurn(ctx, string(body), text, toolCalls, firstNonEmpty(modelVersion, m.model), usage, finish)
			yield(&model.LLMResponse{
				Content:       &genai.Content{Role: genai.RoleModel, Parts: parts},
				ModelVersion:  firstNonEmpty(modelVersion, m.model),
				TurnComplete:  true,
				FinishReason:  finishReason(finish),
				UsageMetadata: usageMetadataFromChatGPT(usage),
				CustomMetadata: map[string]any{
					"provider": m.providerID,
				},
			}, nil)
			return
		}
		if text == "" {
			yield(nil, fmt.Errorf("ChatGPT response returned no text"))
			return
		}
		m.observeTurn(ctx, string(body), text, nil, firstNonEmpty(modelVersion, m.model), usage, finish)
		yield(&model.LLMResponse{
			Content:       genai.NewContentFromText(text, genai.RoleModel),
			ModelVersion:  firstNonEmpty(modelVersion, m.model),
			TurnComplete:  true,
			FinishReason:  finishReason(finish),
			UsageMetadata: usageMetadataFromChatGPT(usage),
			CustomMetadata: map[string]any{
				"provider": m.providerID,
			},
		}, nil)
	}
}

func (m *ChatGPTOAuthModel) applyAuthHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+m.credential.AccessToken)
	if m.credential.AccountID != "" {
		req.Header.Set("ChatGPT-Account-ID", m.credential.AccountID)
	}
	if m.credential.FedRAMP {
		req.Header.Set("X-OpenAI-Fedramp", "true")
	}
}

func (m *ChatGPTOAuthModel) refreshCredential(ctx context.Context) error {
	m.refreshMu.Lock()
	defer m.refreshMu.Unlock()
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", "app_EMoamEEZ73f0CkXaXp7hrann")
	form.Set("refresh_token", m.credential.RefreshToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := m.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("OAuth token refresh returned %s", resp.Status)
	}
	var token map[string]any
	if err := json.Unmarshal(raw, &token); err != nil {
		return fmt.Errorf("OAuth token refresh returned invalid JSON")
	}
	if !nonEmptyTokenString(token, "refresh_token") {
		token["refresh_token"] = m.credential.RefreshToken
	}
	credentialJSON, err := buildChatGPTCredential(token)
	if err != nil {
		return err
	}
	next, err := parseChatGPTCredential(credentialJSON)
	if err != nil {
		return err
	}
	m.credential = next
	if m.credentialUpdater != nil {
		if err := m.credentialUpdater(credentialJSON); err != nil {
			return err
		}
	}
	return nil
}

func (m *ChatGPTOAuthModel) buildRequest(req *model.LLMRequest) (chatGPTResponsesRequest, error) {
	if req == nil {
		return chatGPTResponsesRequest{}, fmt.Errorf("request is required")
	}
	messages, err := buildMessages(req)
	if err != nil {
		return chatGPTResponsesRequest{}, err
	}
	var instructions []string
	items := make([]chatGPTItem, 0, len(messages))
	for _, message := range messages {
		content := strings.TrimSpace(messageContentText(message.Content))
		switch message.Role {
		case "system":
			instructions = append(instructions, content)
		case "assistant":
			if content != "" {
				items = append(items, chatGPTItem{Type: "message", Role: "assistant", Content: []chatGPTContent{{Type: "output_text", Text: content}}})
			}
			for _, call := range message.ToolCalls {
				items = append(items, chatGPTItem{Type: "function_call", CallID: firstNonEmpty(call.ID, "call_"+call.Function.Name), Name: call.Function.Name, Arguments: call.Function.Arguments})
			}
		case "tool":
			items = append(items, chatGPTItem{Type: "function_call_output", CallID: message.ToolCallID, Output: content})
		default:
			if content == "" {
				continue
			}
			items = append(items, chatGPTItem{Type: "message", Role: "user", Content: []chatGPTContent{{Type: "input_text", Text: content}}})
		}
	}
	if len(items) == 0 {
		return chatGPTResponsesRequest{}, fmt.Errorf("message history is empty")
	}
	instructionText := strings.TrimSpace(strings.Join(instructions, "\n\n"))
	if instructionText == "" {
		instructionText = "You are aw, a concise and helpful desktop assistant."
	}
	payload := chatGPTResponsesRequest{
		Model:             firstNonEmpty(req.Model, m.model),
		Instructions:      instructionText,
		Input:             items,
		Tools:             buildChatGPTTools(req),
		ToolChoice:        "auto",
		ParallelToolCalls: false,
		Store:             false,
		Stream:            true,
		Include:           []string{},
	}
	if req.Config != nil && req.Config.Temperature != nil {
		payload.Text = &chatGPTTextOpts{Verbosity: "medium"}
	}
	return payload, nil
}

func buildChatGPTTools(req *model.LLMRequest) []chatGPTTool {
	if req == nil || req.Config == nil {
		return nil
	}
	var tools []chatGPTTool
	for _, t := range req.Config.Tools {
		if t == nil {
			continue
		}
		for _, decl := range t.FunctionDeclarations {
			if decl == nil || strings.TrimSpace(decl.Name) == "" {
				continue
			}
			tools = append(tools, chatGPTTool{
				Type:        "function",
				Name:        decl.Name,
				Description: decl.Description,
				Parameters:  declarationParameters(decl),
			})
		}
	}
	return tools
}

func (m *ChatGPTOAuthModel) readResponsesSSE(body io.Reader, partials bool, yield func(*model.LLMResponse, error) bool) (string, []oaToolCall, string, *chatGPTTokenUsage, string, error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	var full strings.Builder
	var toolCalls []oaToolCall
	var eventName string
	var dataLines []string
	var modelVersion string
	var usage *chatGPTTokenUsage
	finish := "stop"
	flush := func() error {
		if len(dataLines) == 0 {
			return nil
		}
		data := strings.TrimSpace(strings.Join(dataLines, "\n"))
		dataLines = nil
		if data == "" || data == "[DONE]" {
			return nil
		}
		var event map[string]any
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			return nil
		}
		kind, _ := event["type"].(string)
		if kind == "" {
			kind = eventName
		}
		switch kind {
		case "response.output_text.delta":
			delta, _ := event["delta"].(string)
			if delta == "" {
				return nil
			}
			full.WriteString(delta)
			if partials {
				yield(&model.LLMResponse{Content: genai.NewContentFromText(delta, genai.RoleModel), Partial: true}, nil)
			}
		case "response.output_item.done":
			if call, ok := functionCallFromResponseItem(event["item"]); ok {
				toolCalls = append(toolCalls, call)
				return nil
			}
			text := outputTextFromResponseItem(event["item"])
			if text != "" && full.Len() == 0 {
				full.WriteString(text)
			}
		case "response.completed":
			if response, ok := event["response"].(map[string]any); ok {
				if modelName, _ := response["model"].(string); modelName != "" {
					modelVersion = modelName
				}
				usage = tokenUsageFromResponse(response["usage"])
				toolCalls = appendMissingToolCalls(toolCalls, functionCallsFromResponseOutput(response["output"])...)
			}
		case "response.failed", "response.incomplete":
			return errorFromResponseEvent(event)
		}
		return nil
	}
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if err := flush(); err != nil {
				return "", nil, "", nil, "", err
			}
			eventName = ""
			continue
		}
		if strings.HasPrefix(trimmed, "event:") {
			eventName = strings.TrimSpace(strings.TrimPrefix(trimmed, "event:"))
			continue
		}
		if strings.HasPrefix(trimmed, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(trimmed, "data:")))
		}
	}
	if err := scanner.Err(); err != nil {
		return "", nil, "", nil, "", err
	}
	if err := flush(); err != nil {
		return "", nil, "", nil, "", err
	}
	return full.String(), toolCalls, modelVersion, usage, finish, nil
}

func functionCallsFromResponseOutput(raw any) []oaToolCall {
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	calls := make([]oaToolCall, 0)
	for _, item := range items {
		if call, ok := functionCallFromResponseItem(item); ok {
			calls = append(calls, call)
		}
	}
	return calls
}

func functionCallFromResponseItem(raw any) (oaToolCall, bool) {
	item, ok := raw.(map[string]any)
	if !ok {
		return oaToolCall{}, false
	}
	typ, _ := item["type"].(string)
	if typ != "function_call" {
		return oaToolCall{}, false
	}
	name, _ := item["name"].(string)
	arguments, _ := item["arguments"].(string)
	if strings.TrimSpace(name) == "" {
		return oaToolCall{}, false
	}
	id, _ := item["call_id"].(string)
	if id == "" {
		id, _ = item["id"].(string)
	}
	if id == "" {
		id = "call_" + name
	}
	call := oaToolCall{ID: id, Type: "function"}
	call.Function.Name = name
	call.Function.Arguments = arguments
	return call, true
}

func appendMissingToolCalls(existing []oaToolCall, next ...oaToolCall) []oaToolCall {
	seen := map[string]bool{}
	for _, call := range existing {
		seen[firstNonEmpty(call.ID, call.Function.Name)] = true
	}
	for _, call := range next {
		key := firstNonEmpty(call.ID, call.Function.Name)
		if seen[key] {
			continue
		}
		existing = append(existing, call)
		seen[key] = true
	}
	return existing
}

func outputTextFromResponseItem(raw any) string {
	item, ok := raw.(map[string]any)
	if !ok {
		return ""
	}
	content, ok := item["content"].([]any)
	if !ok {
		return ""
	}
	var out strings.Builder
	for _, entry := range content {
		part, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		if typ, _ := part["type"].(string); typ != "output_text" {
			continue
		}
		if text, _ := part["text"].(string); text != "" {
			out.WriteString(text)
		}
	}
	return out.String()
}

func tokenUsageFromResponse(raw any) *chatGPTTokenUsage {
	m, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	usage := &chatGPTTokenUsage{
		InputTokens:  intFromAny(m["input_tokens"]),
		OutputTokens: intFromAny(m["output_tokens"]),
		TotalTokens:  intFromAny(m["total_tokens"]),
	}
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.InputTokens + usage.OutputTokens
	}
	if usage.InputTokens == 0 && usage.OutputTokens == 0 && usage.TotalTokens == 0 {
		return nil
	}
	return usage
}

func intFromAny(value any) int {
	switch v := value.(type) {
	case float64:
		return int(v)
	case int:
		return v
	default:
		return 0
	}
}

func errorFromResponseEvent(event map[string]any) error {
	if response, ok := event["response"].(map[string]any); ok {
		if errObj, ok := response["error"].(map[string]any); ok {
			if msg, _ := errObj["message"].(string); strings.TrimSpace(msg) != "" {
				return errors.New(msg)
			}
		}
	}
	if msg, _ := event["message"].(string); strings.TrimSpace(msg) != "" {
		return errors.New(msg)
	}
	return errors.New("ChatGPT response failed")
}

func (m *ChatGPTOAuthModel) observeTurn(ctx context.Context, requestJSON string, responseText string, toolCalls []oaToolCall, modelName string, usage *chatGPTTokenUsage, finishReason string) {
	if m.observer == nil {
		return
	}
	toolCallsJSON := []byte("[]")
	if len(toolCalls) > 0 {
		toolCallsJSON, _ = json.Marshal(toolCalls)
	}
	turn := domain.LLMTurn{
		RequestJSON:      requestJSON,
		ResponseText:     strings.TrimSpace(responseText),
		ToolCallsJSON:    string(toolCallsJSON),
		Model:            firstNonEmpty(modelName, m.model),
		PromptTokens:     chatGPTUsageInput(usage),
		CompletionTokens: chatGPTUsageOutput(usage),
		FinishReason:     finishReason,
	}
	m.observer(ctx, turn)
}

func usageMetadataFromChatGPT(usage *chatGPTTokenUsage) *genai.GenerateContentResponseUsageMetadata {
	if usage == nil {
		return nil
	}
	total := usage.TotalTokens
	if total == 0 {
		total = usage.InputTokens + usage.OutputTokens
	}
	return &genai.GenerateContentResponseUsageMetadata{
		PromptTokenCount:     int32(usage.InputTokens),
		CandidatesTokenCount: int32(usage.OutputTokens),
		TotalTokenCount:      int32(total),
	}
}

func chatGPTUsageInput(usage *chatGPTTokenUsage) int {
	if usage == nil {
		return 0
	}
	return usage.InputTokens
}

func chatGPTUsageOutput(usage *chatGPTTokenUsage) int {
	if usage == nil {
		return 0
	}
	return usage.OutputTokens
}

var _ model.LLM = (*ChatGPTOAuthModel)(nil)
