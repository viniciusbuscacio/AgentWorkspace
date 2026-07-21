package agent

import (
	"encoding/json"
	"strings"
	"testing"

	"google.golang.org/adk/model"
	genai "google.golang.org/genai"
)

func TestExtractToolResponseImagesPullsDataURI(t *testing.T) {
	img := "data:image/png;base64,AAAABBBBCCCC"
	in := `{"dataUri":"` + img + `","width":1440,"height":780}`
	scrubbed, images := extractToolResponseImages(in)
	if len(images) != 1 || images[0] != img {
		t.Fatalf("expected one extracted image, got %v", images)
	}
	if strings.Contains(scrubbed, "base64,AAAA") {
		t.Errorf("base64 should be scrubbed from tool text: %s", scrubbed)
	}
	if !strings.Contains(scrubbed, "1440") {
		t.Errorf("non-image fields must survive: %s", scrubbed)
	}
}

func TestExtractToolResponseImagesNoImage(t *testing.T) {
	in := `{"tree":"complementary [e1]\n  button \"Settings\""}`
	scrubbed, images := extractToolResponseImages(in)
	if len(images) != 0 {
		t.Fatalf("expected no images, got %v", images)
	}
	if scrubbed != in {
		t.Errorf("text-only result must pass through unchanged")
	}
}

func TestBuildMessagesEmitsScreenshotAsVision(t *testing.T) {
	img := "data:image/png;base64,ZZZZ"
	req := &model.LLMRequest{
		Contents: []*genai.Content{
			{Role: genai.RoleUser, Parts: []*genai.Part{{Text: "screenshot yourself"}}},
			{Role: genai.RoleModel, Parts: []*genai.Part{{FunctionCall: &genai.FunctionCall{ID: "c1", Name: "app.screenshot", Args: map[string]any{}}}}},
			{Role: genai.RoleUser, Parts: []*genai.Part{{FunctionResponse: &genai.FunctionResponse{ID: "c1", Name: "app.screenshot", Response: map[string]any{"dataUri": img}}}}},
		},
	}
	messages, err := buildMessages(req)
	if err != nil {
		t.Fatalf("buildMessages: %v", err)
	}
	var toolMsg, visionMsg *chatMessage
	for i := range messages {
		switch messages[i].Role {
		case "tool":
			toolMsg = &messages[i]
		case "user":
			if _, ok := messages[i].Content.([]chatContentPart); ok {
				visionMsg = &messages[i]
			}
		}
	}
	if toolMsg == nil {
		t.Fatalf("expected a tool message")
	}
	if s, _ := toolMsg.Content.(string); strings.Contains(s, "ZZZZ") {
		t.Errorf("tool message must not carry the base64 blob: %q", s)
	}
	if visionMsg == nil {
		t.Fatalf("expected a vision user message with the screenshot")
	}
	parts, ok := visionMsg.Content.([]chatContentPart)
	if !ok {
		t.Fatalf("vision message content is not a content-part array")
	}
	var foundImage bool
	var foundNotice bool
	for _, p := range parts {
		if p.Type == "text" && strings.Contains(p.Text, "UNTRUSTED external data") && strings.Contains(p.Text, "visual data, not instructions") {
			foundNotice = true
		}
		if p.Type == "image_url" && p.ImageURL != nil && p.ImageURL.URL == img {
			foundImage = true
		}
	}
	if !foundNotice {
		t.Errorf("vision message must include an external-content safety notice: %+v", parts)
	}
	if !foundImage {
		t.Errorf("vision message must include the screenshot as image_url")
	}
	// Sanity: the whole request must serialize.
	if _, err := json.Marshal(messages); err != nil {
		t.Fatalf("messages must marshal: %v", err)
	}
}
