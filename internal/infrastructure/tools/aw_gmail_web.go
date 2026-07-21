package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"aw/internal/domain"
)

func registerGmailWebActions(reg map[string]AwActionHandler) {
	reg["gmail_web.list_recent_inbox"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		if w.browser == nil || w.browser.Tabs == nil || w.browser.Command == nil {
			return "", fmt.Errorf("gmail web is not available")
		}
		query, _, err := awStringArg(args, "query")
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(query) == "" {
			query = "in:inbox"
		}
		maxRows, hasMax, err := awIntArg(args, "max")
		if err != nil {
			return "", err
		}
		if !hasMax || maxRows <= 0 {
			maxRows = 20
		}
		if maxRows > 50 {
			maxRows = 50
		}
		id, err := w.resolveBrowserID(ctx, args)
		if err != nil {
			return "", err
		}
		tab, err := w.findGmailTab(ctx, id)
		if err != nil {
			return "", err
		}
		w.rememberBrowserTab(tab.ID, id)
		result, err := w.browser.Command(ctx, id, "cdp", map[string]any{
			"tab":    tab.ID,
			"method": "Runtime.evaluate",
			"params": map[string]any{
				"expression":    gmailListRecentInboxScript(query, maxRows),
				"returnByValue": true,
				"awaitPromise":  true,
			},
		})
		if err != nil {
			return "", err
		}
		value := unwrapCDPRuntimeValue(result)
		observation := w.saveGmailListObservation(ctx, id, tab, value)
		// Scraped inbox rows (sender/subject/snippet) are attacker-controlled
		// email text — the primary injection surface. Same envelope as
		// browser.cdp: everything stays available, labeled as untrusted data.
		envelope := w.processExternalContent(ctx, value, externalProcessOptions{
			SourceType: domain.ExternalSourceEmail,
			Origin:     "gmail_web.list_recent_inbox",
			Mode:       domain.ExternalContentModePreserveVerbatim,
		})
		return awJSON(map[string]any{
			"browser": id,
			"tab": map[string]any{
				"id":    tab.ID,
				"title": tab.Title,
				"url":   tab.URL,
			},
			"query":       query,
			"result":      envelope,
			"observation": observation,
		})
	}

	reg["gmail_web.delete_listed_inbox_row"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		if w.browser == nil || w.browser.Tabs == nil || w.browser.Command == nil {
			return "", fmt.Errorf("gmail web is not available")
		}
		id, err := w.resolveBrowserID(ctx, args)
		if err != nil {
			return "", err
		}
		target, err := w.gmailListedTarget(ctx, args)
		if err != nil {
			return "", err
		}
		if err := w.requireExternalActionGuard(ctx, "gmail_web.delete_listed_inbox_row", domain.ExternalActionDelete, args); err != nil {
			return "", err
		}
		tab, err := w.findGmailTab(ctx, id)
		if err != nil {
			return "", err
		}
		w.rememberBrowserTab(tab.ID, id)
		result, err := w.browser.Command(ctx, id, "cdp", map[string]any{
			"tab":    tab.ID,
			"method": "Runtime.evaluate",
			"params": map[string]any{
				"expression":    gmailDeleteListedScript(target),
				"returnByValue": true,
				"awaitPromise":  true,
			},
		})
		if err != nil {
			return "", err
		}
		value := unwrapCDPRuntimeValue(result)
		// The matched sender/subject/date echoed back are scraped page text —
		// wrap them like every other web read.
		envelope := w.processExternalContent(ctx, value, externalProcessOptions{
			SourceType: domain.ExternalSourceEmail,
			Origin:     "gmail_web.delete_listed_inbox_row",
			Mode:       domain.ExternalContentModePreserveVerbatim,
		})
		return awJSON(map[string]any{
			"browser": id,
			"tab": map[string]any{
				"id":    tab.ID,
				"title": tab.Title,
				"url":   tab.URL,
			},
			"target": target,
			"result": envelope,
		})
	}

	reg["gmail_web.delete_one_from_inbox"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		if w.browser == nil || w.browser.Tabs == nil || w.browser.Command == nil {
			return "", fmt.Errorf("gmail web is not available")
		}
		sender, err := awRequiredStringArg(args, "sender")
		if err != nil {
			return "", err
		}
		query, _, err := awStringArg(args, "query")
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(query) == "" {
			query = fmt.Sprintf("in:inbox %q", sender)
		}
		id, err := w.resolveBrowserID(ctx, args)
		if err != nil {
			return "", err
		}
		if err := w.requireExternalActionGuard(ctx, "gmail_web.delete_one_from_inbox", domain.ExternalActionDelete, args); err != nil {
			return "", err
		}
		tab, err := w.findGmailTab(ctx, id)
		if err != nil {
			return "", err
		}
		w.rememberBrowserTab(tab.ID, id)
		result, err := w.browser.Command(ctx, id, "cdp", map[string]any{
			"tab":    tab.ID,
			"method": "Runtime.evaluate",
			"params": map[string]any{
				"expression":    gmailDeleteOneScript(sender, query),
				"returnByValue": true,
				"awaitPromise":  true,
			},
		})
		if err != nil {
			return "", err
		}
		value := unwrapCDPRuntimeValue(result)
		w.logGmailWebDelete(ctx, id, sender, value)
		envelope := w.processExternalContent(ctx, value, externalProcessOptions{
			SourceType: domain.ExternalSourceEmail,
			Origin:     "gmail_web.delete_one_from_inbox",
			Mode:       domain.ExternalContentModePreserveVerbatim,
		})
		return awJSON(map[string]any{
			"browser": id,
			"tab": map[string]any{
				"id":    tab.ID,
				"title": tab.Title,
				"url":   tab.URL,
			},
			"sender": sender,
			"query":  query,
			"result": envelope,
		})
	}
}

func (w *workspace) saveGmailListObservation(ctx context.Context, browserID string, tab domain.BrowserTab, value any) any {
	if w.webRead == nil || w.webRead.Save == nil {
		return nil
	}
	chatID := strings.TrimSpace(domain.ChatSessionScope(ctx))
	if chatID == "" {
		return nil
	}
	payload := map[string]any{
		"kind":  "gmail-list",
		"items": gmailRowsFromValue(value),
	}
	result, err := w.webRead.Save(ctx, WebReadSaveInput{
		ChatID:  chatID,
		RunID:   domain.ExternalTaintScope(ctx),
		Browser: browserID,
		TabID:   tab.ID,
		SiteKey: "https://mail.google.com",
		URL:     tab.URL,
		Title:   tab.Title,
		Payload: payload,
	})
	if err != nil {
		return map[string]any{"saved": false, "error": err.Error()}
	}
	// Return save metadata only: the observation view echoes the full payload
	// (the raw scraped rows), which would bypass the external_safety envelope
	// the caller wraps around the same rows.
	summary := map[string]any{"saved": true}
	if view, ok := result.(map[string]any); ok {
		for _, key := range []string{"id", "capturedAt", "siteKey"} {
			if v, exists := view[key]; exists {
				summary[key] = v
			}
		}
	} else if view, err := json.Marshal(result); err == nil {
		var m map[string]any
		if json.Unmarshal(view, &m) == nil {
			for _, key := range []string{"id", "capturedAt", "siteKey"} {
				if v, exists := m[key]; exists {
					summary[key] = v
				}
			}
		}
	}
	return summary
}

func gmailRowsFromValue(value any) []any {
	m, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	rows, ok := m["rows"].([]any)
	if ok {
		return rows
	}
	typedRows, ok := m["rows"].([]map[string]any)
	if !ok {
		return nil
	}
	out := make([]any, 0, len(typedRows))
	for _, row := range typedRows {
		out = append(out, row)
	}
	return out
}

func (w *workspace) gmailListedTarget(ctx context.Context, args map[string]any) (map[string]any, error) {
	target := map[string]any{}
	for _, key := range []string{"threadId", "sender", "senderName", "email", "subject", "date", "query"} {
		if value, _, err := awStringArg(args, key); err != nil {
			return nil, err
		} else if strings.TrimSpace(value) != "" {
			target[key] = strings.TrimSpace(value)
		}
	}
	if ordinal, has, err := awIntArg(args, "ordinal"); err != nil {
		return nil, err
	} else if has && ordinal > 0 {
		target["ordinal"] = ordinal
		if row, ok, err := w.gmailObservationRow(ctx, args, ordinal); err != nil {
			return nil, err
		} else if ok {
			for key, value := range row {
				if _, exists := target[key]; !exists {
					target[key] = value
				}
			}
		}
	}
	if strings.TrimSpace(argString(target["query"])) == "" {
		target["query"] = "in:inbox"
	}
	if strings.TrimSpace(argString(target["threadId"])) == "" && strings.TrimSpace(argString(target["subject"])) == "" && strings.TrimSpace(argString(target["sender"])) == "" && strings.TrimSpace(argString(target["email"])) == "" {
		return nil, fmt.Errorf("delete_listed_inbox_row needs ordinal from a saved Gmail list, threadId, subject, sender, or email")
	}
	return target, nil
}

func (w *workspace) gmailObservationRow(ctx context.Context, args map[string]any, ordinal int) (map[string]any, bool, error) {
	if w.webRead == nil || w.webRead.Latest == nil {
		return nil, false, nil
	}
	chatID, _, err := awStringArg(args, "chatId")
	if err != nil {
		return nil, false, err
	}
	if strings.TrimSpace(chatID) == "" {
		chatID = domain.ChatSessionScope(ctx)
	}
	browser, _, err := awStringArg(args, "browser")
	if err != nil {
		return nil, false, err
	}
	obs, ok, err := w.webRead.Latest(ctx, chatID, browser, "https://mail.google.com", "")
	if err != nil || !ok {
		return nil, ok, err
	}
	obsMap, ok := obs.(map[string]any)
	if !ok {
		return nil, false, nil
	}
	payload, ok := obsMap["payload"].(map[string]any)
	if !ok {
		return nil, false, nil
	}
	items, ok := payload["items"].([]any)
	if !ok {
		return nil, false, nil
	}
	for _, raw := range items {
		row, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if intFromAny(row["ordinal"]) == ordinal {
			return row, true, nil
		}
	}
	return nil, false, nil
}

func intFromAny(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}

func (w *workspace) logGmailWebDelete(ctx context.Context, browserID string, sender string, result any) {
	attrs := map[string]any{
		"browser":     browserID,
		"sender.hash": hashForToolLog(sender),
	}
	if value, ok := result.(map[string]any); ok {
		for _, key := range []string{"status", "deleted", "beforeCount", "afterCount"} {
			if v, exists := value[key]; exists {
				attrs[key] = v
			}
		}
	}
	w.logAction(ctx, ActionLogEvent{
		Action:     "gmail_web.delete_one_from_inbox",
		Event:      "gmail_web.delete_one.completed",
		Status:     "ok",
		Attributes: attrs,
	})
}

func unwrapCDPRuntimeValue(result any) any {
	outer, ok := result.(map[string]any)
	if !ok {
		return result
	}
	inner, ok := outer["result"].(map[string]any)
	if !ok {
		return result
	}
	if value, ok := inner["value"]; ok {
		return value
	}
	return result
}

func (w *workspace) findGmailTab(ctx context.Context, browserID string) (domain.BrowserTab, error) {
	result, err := w.browser.Tabs(ctx, browserID)
	if err != nil {
		return domain.BrowserTab{}, err
	}
	tabs := normalizeBrowserTabs(result)
	for _, tab := range tabs {
		if strings.Contains(strings.ToLower(tab.URL), "mail.google.com") {
			return tab, nil
		}
	}
	return domain.BrowserTab{}, fmt.Errorf("no Gmail tab found in %s; open Gmail first or pass the correct browser", browserID)
}

func normalizeBrowserTabs(result any) []domain.BrowserTab {
	switch tabs := result.(type) {
	case []domain.BrowserTab:
		return tabs
	case []map[string]any:
		out := make([]domain.BrowserTab, 0, len(tabs))
		for _, tab := range tabs {
			out = append(out, domain.BrowserTab{
				ID:    strings.TrimSpace(argString(tab["id"])),
				Title: strings.TrimSpace(argString(tab["title"])),
				URL:   strings.TrimSpace(argString(tab["url"])),
			})
		}
		return out
	case []any:
		out := make([]domain.BrowserTab, 0, len(tabs))
		for _, raw := range tabs {
			tab, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			out = append(out, domain.BrowserTab{
				ID:    strings.TrimSpace(argString(tab["id"])),
				Title: strings.TrimSpace(argString(tab["title"])),
				URL:   strings.TrimSpace(argString(tab["url"])),
			})
		}
		return out
	default:
		return nil
	}
}

func gmailDeleteOneScript(sender string, query string) string {
	return fmt.Sprintf(`(async () => {
  const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
  const senderNeedle = %s.toLowerCase();
  const query = %s;
  const visible = (el) => {
    if (!el) return false;
    const style = getComputedStyle(el);
    const rect = el.getBoundingClientRect();
    return style.visibility !== "hidden" && style.display !== "none" && rect.width > 0 && rect.height > 0;
  };
  const hasLayout = (el) => {
    if (!el) return false;
    const rect = el.getBoundingClientRect();
    return rect.width > 0 && rect.height > 0;
  };
  const activate = (el) => {
    for (const type of ["pointerdown", "mousedown", "pointerup", "mouseup", "click"]) {
      el.dispatchEvent(new MouseEvent(type, {bubbles: true, cancelable: true, view: window}));
    }
  };
  const setNativeValue = (input, value) => {
    const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")?.set;
    if (setter) setter.call(input, value);
    else input.value = value;
    input.dispatchEvent(new InputEvent("input", {bubbles: true, inputType: "insertText", data: value}));
    input.dispatchEvent(new Event("change", {bubbles: true}));
  };
  const rowInfo = () => Array.from(document.querySelectorAll('tr[role="row"]'))
    .map((row, index) => {
      const senderEl = row.querySelector('.yW span[email], .yW span[name], .yW, [email]');
      const subjectEl = row.querySelector('.bog, [data-thread-id] .bog');
      const dateEl = row.querySelector('.xW, [aria-label*=", 20"], [title*=", 20"]');
      const text = (row.innerText || "").replace(/\s+/g, " ").trim();
      const senderText = ((senderEl && (senderEl.getAttribute("email") || senderEl.getAttribute("name") || senderEl.textContent)) || "").replace(/\s+/g, " ").trim();
      return {index, row, text, sender: senderText, subject: (subjectEl?.textContent || "").trim(), date: (dateEl?.textContent || dateEl?.getAttribute("title") || "").trim()};
    })
    .filter((item) => item.text && item.text.toLowerCase().includes(senderNeedle));
  const visibleRowInfo = () => rowInfo().filter((item) => hasLayout(item.row));
  const search = document.querySelector('input[aria-label="Search mail"], input[placeholder*="Search"], form[role="search"] input');
  if (!search) return {status: "blocked", reason: "gmail_search_box_not_found"};
  setNativeValue(search, query);
  search.dispatchEvent(new KeyboardEvent("keydown", {bubbles: true, cancelable: true, key: "Enter", code: "Enter", keyCode: 13, which: 13}));
  search.dispatchEvent(new KeyboardEvent("keyup", {bubbles: true, cancelable: true, key: "Enter", code: "Enter", keyCode: 13, which: 13}));
  const form = search.closest("form");
  if (form) form.dispatchEvent(new Event("submit", {bubbles: true, cancelable: true}));
  await sleep(2500);
  let matches = visibleRowInfo();
  if (!matches.length) {
    await sleep(2000);
    matches = visibleRowInfo();
  }
  if (!matches.length) {
    const virtualMatch = rowInfo()[0];
    if (virtualMatch?.row) {
      virtualMatch.row.scrollIntoView({block: "center", inline: "nearest"});
      await sleep(900);
      matches = visibleRowInfo();
    }
  }
  if (!matches.length) return {status: "none_found", deleted: false, beforeCount: 0};
  const target = matches[0];
  const checkbox = target.row.querySelector('[role="checkbox"], .T-Jo');
  if (!checkbox) return {status: "blocked", reason: "row_checkbox_not_found", beforeCount: matches.length, target: {sender: target.sender, subject: target.subject, date: target.date}};
  if (checkbox.getAttribute("aria-checked") !== "true") activate(checkbox);
  await sleep(500);
  const isDeleteControl = (el) => {
      const label = ((el.getAttribute("aria-label") || "") + " " + (el.getAttribute("data-tooltip") || "") + " " + (el.getAttribute("title") || "")).toLowerCase();
      return /\bdelete\b|excluir|mover para a lixeira|move to trash/.test(label) && !/^\s*lixeira\s*$|^\s*trash\s*$/.test(label);
  };
  const rowDeleteCandidates = Array.from(target.row.querySelectorAll('[aria-label], [data-tooltip], [title], [jsaction]'));
  const pageDeleteCandidates = Array.from(document.querySelectorAll('[aria-label], [data-tooltip], [title]'));
  const deleteButton =
    rowDeleteCandidates.filter(visible).find(isDeleteControl) ||
    rowDeleteCandidates.find(isDeleteControl) ||
    pageDeleteCandidates.filter(visible).find(isDeleteControl) ||
    pageDeleteCandidates.find(isDeleteControl);
  if (!deleteButton) return {status: "blocked", reason: "delete_button_not_found", beforeCount: matches.length, target: {sender: target.sender, subject: target.subject, date: target.date}};
  activate(deleteButton);
  await sleep(1800);
  const after = rowInfo();
  return {
    status: after.length < matches.length ? "deleted" : "delete_clicked",
    deleted: after.length < matches.length,
    beforeCount: matches.length,
    afterCount: after.length,
    target: {sender: target.sender, subject: target.subject, date: target.date}
  };
})()`, jsLiteralString(sender), jsLiteralString(query))
}

func gmailListRecentInboxScript(query string, maxRows int) string {
	return fmt.Sprintf(`(async () => {
  const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
  const query = %s;
  const maxRows = %d;
  const norm = (value) => String(value || "").replace(/\s+/g, " ").trim();
  const hasLayout = (el) => {
    if (!el) return false;
    const style = getComputedStyle(el);
    const rect = el.getBoundingClientRect();
    return style.visibility !== "hidden" && style.display !== "none" && rect.width > 0 && rect.height > 0;
  };
  const setNativeValue = (input, value) => {
    const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")?.set;
    if (setter) setter.call(input, value);
    else input.value = value;
    input.dispatchEvent(new InputEvent("input", {bubbles: true, inputType: "insertText", data: value}));
    input.dispatchEvent(new Event("change", {bubbles: true}));
  };
  const readRows = () => Array.from(document.querySelectorAll('tr[role="row"]'))
    .filter(hasLayout)
    .map((row, index) => {
      const text = norm(row.innerText || row.textContent);
      if (!text || text.length < 8) return null;
      const senderEl = row.querySelector('.yW span[email], .yW span[name], .yW, [email]');
      const subjectEl = row.querySelector('.bog, [data-thread-id] .bog, [data-thread-id]');
      const dateEl = row.querySelector('.xW, [aria-label*=", 20"], [title*=", 20"]');
      const idEl = row.querySelector('[data-thread-id], [data-legacy-thread-id], [data-legacy-message-id]');
      const sender = norm((senderEl && (senderEl.getAttribute("email") || senderEl.getAttribute("name") || senderEl.textContent)) || "");
      const senderName = norm((senderEl && (senderEl.getAttribute("name") || senderEl.textContent)) || "");
      const email = norm((senderEl && senderEl.getAttribute("email")) || "");
      const subject = norm(subjectEl?.textContent || "");
      const date = norm(dateEl?.textContent || dateEl?.getAttribute("title") || "");
      const unread = /\bzE\b/.test(String(row.className || "")) || /não lida|nao lida|unread/i.test(text.slice(0, 80));
      return {
        ordinal: index + 1,
        sender,
        senderName,
        email,
        subject,
        date,
        unread,
        snippet: text.slice(0, 260),
        threadId: idEl?.getAttribute("data-thread-id") || idEl?.getAttribute("data-legacy-thread-id") || "",
      };
    })
    .filter(Boolean)
    .filter((row) => row.sender || row.subject || row.snippet)
    .slice(0, maxRows);
  const search = document.querySelector('input[aria-label="Search mail"], input[placeholder*="Search"], form[role="search"] input');
  if (!search) return {status: "blocked", reason: "gmail_search_box_not_found", rows: []};
  setNativeValue(search, query);
  search.dispatchEvent(new KeyboardEvent("keydown", {bubbles: true, cancelable: true, key: "Enter", code: "Enter", keyCode: 13, which: 13}));
  search.dispatchEvent(new KeyboardEvent("keyup", {bubbles: true, cancelable: true, key: "Enter", code: "Enter", keyCode: 13, which: 13}));
  const form = search.closest("form");
  if (form) form.dispatchEvent(new Event("submit", {bubbles: true, cancelable: true}));
  await sleep(2500);
  let rows = readRows();
  if (!rows.length) {
    await sleep(1500);
    rows = readRows();
  }
  return {status: "ok", query, count: rows.length, rows, title: document.title, url: location.href};
})()`, jsLiteralString(query), maxRows)
}

func gmailDeleteListedScript(target map[string]any) string {
	data, _ := json.Marshal(target)
	return fmt.Sprintf(`(async () => {
  const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
  const target = %s;
  const norm = (value) => String(value || "").replace(/\s+/g, " ").trim();
  const lower = (value) => norm(value).toLowerCase();
  const hasLayout = (el) => {
    if (!el) return false;
    const style = getComputedStyle(el);
    const rect = el.getBoundingClientRect();
    return style.visibility !== "hidden" && style.display !== "none" && rect.width > 0 && rect.height > 0;
  };
  const activate = (el) => {
    for (const type of ["pointerdown", "mousedown", "pointerup", "mouseup", "click"]) {
      el.dispatchEvent(new MouseEvent(type, {bubbles: true, cancelable: true, view: window}));
    }
  };
  const setNativeValue = (input, value) => {
    const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")?.set;
    if (setter) setter.call(input, value);
    else input.value = value;
    input.dispatchEvent(new InputEvent("input", {bubbles: true, inputType: "insertText", data: value}));
    input.dispatchEvent(new Event("change", {bubbles: true}));
  };
  const rowInfo = () => Array.from(document.querySelectorAll('tr[role="row"]'))
    .filter(hasLayout)
    .map((row, index) => {
      const text = norm(row.innerText || row.textContent);
      const senderEl = row.querySelector('.yW span[email], .yW span[name], .yW, [email]');
      const subjectEl = row.querySelector('.bog, [data-thread-id] .bog, [data-thread-id]');
      const dateEl = row.querySelector('.xW, [aria-label*=", 20"], [title*=", 20"]');
      const idEl = row.querySelector('[data-thread-id], [data-legacy-thread-id], [data-legacy-message-id]');
      return {
        index,
        row,
        text,
        sender: norm((senderEl && (senderEl.getAttribute("email") || senderEl.getAttribute("name") || senderEl.textContent)) || ""),
        senderName: norm((senderEl && (senderEl.getAttribute("name") || senderEl.textContent)) || ""),
        email: norm((senderEl && senderEl.getAttribute("email")) || ""),
        subject: norm(subjectEl?.textContent || ""),
        date: norm(dateEl?.textContent || dateEl?.getAttribute("title") || ""),
        threadId: idEl?.getAttribute("data-thread-id") || idEl?.getAttribute("data-legacy-thread-id") || "",
      };
    })
    .filter((item) => item.text);
  const matchesTarget = (item) => {
    if (target.threadId && item.threadId && String(item.threadId).includes(String(target.threadId))) return true;
    const subject = lower(target.subject);
    const sender = lower(target.sender || target.senderName || target.email);
    const date = lower(target.date);
    const haystack = lower([item.sender, item.senderName, item.email, item.subject, item.date, item.text].join(" "));
    if (subject && !haystack.includes(subject.slice(0, Math.min(80, subject.length)))) return false;
    if (sender && !haystack.includes(sender.slice(0, Math.min(40, sender.length)))) return false;
    if (date && !haystack.includes(date)) return false;
    return Boolean(subject || sender || target.threadId);
  };
  const search = document.querySelector('input[aria-label="Search mail"], input[placeholder*="Search"], form[role="search"] input');
  if (!search) return {status: "blocked", reason: "gmail_search_box_not_found"};
  const query = target.query || "in:inbox";
  setNativeValue(search, query);
  search.dispatchEvent(new KeyboardEvent("keydown", {bubbles: true, cancelable: true, key: "Enter", code: "Enter", keyCode: 13, which: 13}));
  search.dispatchEvent(new KeyboardEvent("keyup", {bubbles: true, cancelable: true, key: "Enter", code: "Enter", keyCode: 13, which: 13}));
  const form = search.closest("form");
  if (form) form.dispatchEvent(new Event("submit", {bubbles: true, cancelable: true}));
  await sleep(2200);
  let rows = rowInfo();
  let match = rows.find(matchesTarget);
  if (!match) {
    await sleep(1200);
    rows = rowInfo();
    match = rows.find(matchesTarget);
  }
  if (!match) return {status: "none_found", deleted: false, beforeCount: rows.length, reason: "target_not_visible"};
  const beforeCount = rows.filter(matchesTarget).length;
  const checkbox = match.row.querySelector('[role="checkbox"], .T-Jo');
  if (!checkbox) return {status: "blocked", reason: "row_checkbox_not_found", beforeCount, target: {sender: match.sender, subject: match.subject, date: match.date, threadId: match.threadId}};
  if (checkbox.getAttribute("aria-checked") !== "true") activate(checkbox);
  await sleep(350);
  const isDeleteControl = (el) => {
    const label = ((el.getAttribute("aria-label") || "") + " " + (el.getAttribute("data-tooltip") || "") + " " + (el.getAttribute("title") || "")).toLowerCase();
    return /\bdelete\b|excluir|mover para a lixeira|move to trash/.test(label) && !/^\s*lixeira\s*$|^\s*trash\s*$/.test(label);
  };
  const rowDeleteCandidates = Array.from(match.row.querySelectorAll('[aria-label], [data-tooltip], [title], [jsaction]'));
  const pageDeleteCandidates = Array.from(document.querySelectorAll('[aria-label], [data-tooltip], [title]'));
  const deleteButton =
    rowDeleteCandidates.filter(hasLayout).find(isDeleteControl) ||
    rowDeleteCandidates.find(isDeleteControl) ||
    pageDeleteCandidates.filter(hasLayout).find(isDeleteControl) ||
    pageDeleteCandidates.find(isDeleteControl);
  if (!deleteButton) return {status: "blocked", reason: "delete_button_not_found", beforeCount, target: {sender: match.sender, subject: match.subject, date: match.date, threadId: match.threadId}};
  activate(deleteButton);
  await sleep(1600);
  const afterCount = rowInfo().filter(matchesTarget).length;
  return {
    status: afterCount < beforeCount ? "deleted" : "delete_clicked",
    deleted: afterCount < beforeCount,
    beforeCount,
    afterCount,
    target: {sender: match.sender, subject: match.subject, date: match.date, threadId: match.threadId}
  };
})()`, string(data))
}

func jsLiteralString(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}
