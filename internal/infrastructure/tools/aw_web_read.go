package tools

import (
	"context"
	"fmt"
	"strings"

	"aw/internal/domain"
)

type WebReadSaveInput struct {
	ChatID  string
	RunID   string
	Browser string
	TabID   string
	SiteKey string
	URL     string
	Title   string
	Payload any
}

type WebReadFuncs struct {
	Save   func(ctx context.Context, input WebReadSaveInput) (any, error)
	Latest func(ctx context.Context, chatID, browser, siteKey, url string) (any, bool, error)
}

func registerWebReadActions(reg map[string]AwActionHandler) {
	reg["web_read.observation.save"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		if w.webRead == nil || w.webRead.Save == nil {
			return "", fmt.Errorf("web_read is not available")
		}
		chatID, _, err := awStringArg(args, "chatId")
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(chatID) == "" {
			chatID = domain.ChatSessionScope(ctx)
		}
		runID, _, err := awStringArg(args, "runId")
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(runID) == "" {
			runID = domain.ExternalTaintScope(ctx)
		}
		browser, err := awRequiredStringArg(args, "browser")
		if err != nil {
			return "", err
		}
		siteKey, _, err := awStringArg(args, "siteKey")
		if err != nil {
			return "", err
		}
		rawURL, _, err := awStringArg(args, "url")
		if err != nil {
			return "", err
		}
		tabID, _, err := awStringArg(args, "tabId")
		if err != nil {
			return "", err
		}
		title, _, err := awStringArg(args, "title")
		if err != nil {
			return "", err
		}
		payload, ok := args["payload"]
		if !ok || payload == nil {
			return "", fmt.Errorf("payload is required")
		}
		result, err := w.webRead.Save(ctx, WebReadSaveInput{
			ChatID:  chatID,
			RunID:   runID,
			Browser: browser,
			TabID:   tabID,
			SiteKey: siteKey,
			URL:     rawURL,
			Title:   title,
			Payload: payload,
		})
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["web_read.observation.latest"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		if w.webRead == nil || w.webRead.Latest == nil {
			return "", fmt.Errorf("web_read is not available")
		}
		chatID, _, err := awStringArg(args, "chatId")
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(chatID) == "" {
			chatID = domain.ChatSessionScope(ctx)
		}
		browser, _, err := awStringArg(args, "browser")
		if err != nil {
			return "", err
		}
		siteKey, _, err := awStringArg(args, "siteKey")
		if err != nil {
			return "", err
		}
		rawURL, _, err := awStringArg(args, "url")
		if err != nil {
			return "", err
		}
		result, ok, err := w.webRead.Latest(ctx, chatID, browser, siteKey, rawURL)
		if err != nil {
			return "", err
		}
		response := map[string]any{"found": ok, "observation": result}
		if ok {
			response["fastPathRequired"] = true
			response["guidance"] = "A recent observation matched. Resolve the user's reference against observation.payload and attempt one focused action using the saved handle before calling subagent.run, browser.snapshot, browser.tabs, or broad browser.cdp. Re-observe only if the handle is ambiguous, stale, missing a usable selector/ref, or the focused action fails."
		} else {
			response["guidance"] = "No recent observation matched. Use the normal web-read path to observe the site and save fresh handles."
		}
		return awJSON(response)
	}
}
