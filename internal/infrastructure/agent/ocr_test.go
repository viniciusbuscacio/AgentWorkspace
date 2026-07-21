package agent

import (
	"context"
	"iter"
	"strings"
	"testing"

	"aw/internal/domain"
	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

type captureLLM struct {
	gotReq *model.LLMRequest
	text   string
}

func (c *captureLLM) Name() string { return "capture" }

func (c *captureLLM) GenerateContent(_ context.Context, req *model.LLMRequest, _ bool) iter.Seq2[*model.LLMResponse, error] {
	c.gotReq = req
	text := c.text
	return func(yield func(*model.LLMResponse, error) bool) {
		yield(&model.LLMResponse{Content: genai.NewContentFromText(text, genai.RoleModel)}, nil)
	}
}

func TestTranscribeImageBuildsIsolatedRequest(t *testing.T) {
	captured := &captureLLM{text: "  hello from the image  "}
	r := NewRuntimeWithFactory(func(_ context.Context, _ ModelConfig) (model.LLM, error) { return captured, nil })
	cfg := domain.ModelConfig{AuthType: "api-key", Model: "vision"}

	out, err := r.TranscribeImage(context.Background(), cfg, []byte{0x1, 0x2, 0x3}, "image/png")
	if err != nil {
		t.Fatalf("TranscribeImage: %v", err)
	}
	if out != "hello from the image" {
		t.Fatalf("transcription = %q", out)
	}
	if captured.gotReq == nil {
		t.Fatal("no request captured")
	}

	// Isolation invariant: the OCR call must carry NO tools in any location.
	if len(captured.gotReq.Tools) != 0 {
		t.Fatalf("OCR request must have no LLMRequest.Tools, got %d", len(captured.gotReq.Tools))
	}
	if captured.gotReq.Config != nil && len(captured.gotReq.Config.Tools) != 0 {
		t.Fatalf("OCR request must have no Config.Tools, got %d", len(captured.gotReq.Config.Tools))
	}

	// Must contain the fixed transcription prompt and exactly the image bytes.
	var hasPrompt, hasImage bool
	for _, content := range captured.gotReq.Contents {
		for _, p := range content.Parts {
			if p.Text != "" && strings.Contains(p.Text, "text-extraction engine") {
				hasPrompt = true
			}
			if p.InlineData != nil && len(p.InlineData.Data) == 3 {
				hasImage = true
			}
		}
	}
	if !hasPrompt {
		t.Error("OCR request missing the fixed transcription prompt")
	}
	if !hasImage {
		t.Error("OCR request missing the image inline data")
	}
}

func TestTranscribeImageRejectsEmpty(t *testing.T) {
	r := NewRuntimeWithFactory(func(_ context.Context, _ ModelConfig) (model.LLM, error) {
		return &captureLLM{}, nil
	})
	cfg := domain.ModelConfig{AuthType: "api-key", Model: "vision"}
	if _, err := r.TranscribeImage(context.Background(), cfg, nil, "image/png"); err == nil {
		t.Fatal("empty image should error")
	}
}

func TestBuildMessagesUserInlineDataBecomesImageURL(t *testing.T) {
	req := &model.LLMRequest{
		Contents: []*genai.Content{{
			Role: genai.RoleUser,
			Parts: []*genai.Part{
				genai.NewPartFromText("read this"),
				{InlineData: &genai.Blob{MIMEType: "image/png", Data: []byte{0xAA, 0xBB}}},
			},
		}},
	}
	messages, err := buildMessages(req)
	if err != nil {
		t.Fatalf("buildMessages: %v", err)
	}
	var foundImage, foundText bool
	for _, m := range messages {
		parts, ok := m.Content.([]chatContentPart)
		if !ok {
			continue
		}
		for _, p := range parts {
			if p.Type == "image_url" && p.ImageURL != nil && strings.HasPrefix(p.ImageURL.URL, "data:image/png;base64,") {
				foundImage = true
			}
			if p.Type == "text" && p.Text == "read this" {
				foundText = true
			}
		}
	}
	if !foundImage {
		t.Fatal("user inline image should become an image_url content part")
	}
	if !foundText {
		t.Fatal("accompanying text should be preserved alongside the image")
	}
}
