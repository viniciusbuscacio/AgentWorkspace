package application

import (
	"fmt"
	"html"
	"net/url"
	"strings"

	"aw/internal/domain"
)

type BrowserSnapshotInput struct {
	RefreshedAt string
	Browsers    []BrowserSnapshotBrowser
}

type BrowserSnapshotBrowser struct {
	ID     string
	Name   string
	Status domain.BrowserStatus
	Tabs   []domain.BrowserTab
	Error  string
}

func BrowserSnapshotPromptBlock(input BrowserSnapshotInput) string {
	if len(input.Browsers) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("## Browser snapshot\n\n")
	b.WriteString("Fresh turn-start snapshot of connected browser tabs. Use it directly for tab questions; call browser tools only for refreshes or actions.\n")
	if strings.TrimSpace(input.RefreshedAt) != "" {
		b.WriteString("Refreshed at: ")
		b.WriteString(strings.TrimSpace(input.RefreshedAt))
		b.WriteString("\n")
	}
	for _, browser := range input.Browsers {
		name := firstNonEmpty(strings.TrimSpace(browser.Name), strings.TrimSpace(browser.ID), "Browser")
		b.WriteString("\n### ")
		b.WriteString(ScrubChatSecrets(name))
		b.WriteString("\n")
		if strings.TrimSpace(browser.Error) != "" {
			b.WriteString("- Error: ")
			b.WriteString(ScrubChatSecrets(strings.TrimSpace(browser.Error)))
			b.WriteString("\n")
			continue
		}
		profile := browserProfileLabel(browser.Status)
		b.WriteString("- Profile: ")
		b.WriteString(profile)
		b.WriteString("\n")
		if notice := strings.TrimSpace(browser.Status.Notice); notice != "" {
			b.WriteString("- Notice: ")
			b.WriteString(ScrubChatSecrets(notice))
			b.WriteString("\n")
		}
		b.WriteString(fmt.Sprintf("- Open tabs: %d\n", len(browser.Tabs)))
		if len(browser.Tabs) == 0 {
			continue
		}
		limit := len(browser.Tabs)
		if limit > 30 {
			limit = 30
		}
		for i := 0; i < limit; i++ {
			tab := browser.Tabs[i]
			title := strings.TrimSpace(promptText(html.UnescapeString(tab.Title)))
			if title == "" {
				title = "Untitled"
			}
			b.WriteString(fmt.Sprintf("  %d. %s", i+1, title))
			if origin := safeBrowserOrigin(tab.URL); origin != "" {
				b.WriteString(" — ")
				b.WriteString(origin)
			}
			b.WriteString("\n")
		}
		if len(browser.Tabs) > limit {
			b.WriteString(fmt.Sprintf("  ... %d more tabs omitted.\n", len(browser.Tabs)-limit))
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func browserProfileLabel(status domain.BrowserStatus) string {
	profileDir := strings.TrimSpace(status.ProfileDir)
	if profileDir == "" {
		return "unknown"
	}
	if strings.Contains(strings.ReplaceAll(profileDir, "\\", "/"), "/browser-profiles/") {
		return "aw"
	}
	return "unknown"
}

func safeBrowserOrigin(raw string) string {
	raw = strings.TrimSpace(promptText(raw))
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" {
		return ""
	}
	if parsed.Scheme == "about" || parsed.Scheme == "chrome" || parsed.Scheme == "edge" {
		return parsed.Scheme + ":" + parsed.Opaque
	}
	if parsed.Host == "" {
		return parsed.Scheme + ":"
	}
	return parsed.Scheme + "://" + parsed.Host
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
