package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// session is a minimal Chrome DevTools Protocol client over the page
// websocket: send a command, wait for the matching response id, skip events.
type session struct {
	conn   *websocket.Conn
	nextID int
}

const cdpCallTimeout = 30 * time.Second

func dialPage(ctx context.Context, wsURL string) (*session, error) {
	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	conn, resp, err := dialer.DialContext(contextOrBackground(ctx), wsURL, nil)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		return nil, fmt.Errorf("dial CDP websocket: %w", err)
	}
	return &session{conn: conn}, nil
}

func (s *session) close() {
	_ = s.conn.Close()
}

type cdpResponse struct {
	ID     int             `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// call sends one CDP command and blocks until its response arrives.
func (s *session) call(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
	s.nextID++
	id := s.nextID
	payload := map[string]any{"id": id, "method": method}
	if params != nil {
		payload["params"] = params
	}
	deadline := time.Now().Add(cdpCallTimeout)
	if ctxDeadline, ok := contextOrBackground(ctx).Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	_ = s.conn.SetWriteDeadline(deadline)
	if err := s.conn.WriteJSON(payload); err != nil {
		return nil, fmt.Errorf("send %s: %w", method, err)
	}
	for {
		if err := contextOrBackground(ctx).Err(); err != nil {
			return nil, err
		}
		_ = s.conn.SetReadDeadline(deadline)
		var response cdpResponse
		if err := s.conn.ReadJSON(&response); err != nil {
			return nil, fmt.Errorf("read %s response: %w", method, err)
		}
		if response.ID != id {
			continue // an event or an older response — skip
		}
		if response.Error != nil {
			return nil, fmt.Errorf("%s failed: %s", method, response.Error.Message)
		}
		return response.Result, nil
	}
}

// evaluate runs a JS expression in the page and returns its JSON value.
func (s *session) evaluate(ctx context.Context, expression string) (json.RawMessage, error) {
	result, err := s.call(ctx, "Runtime.evaluate", map[string]any{
		"expression":    expression,
		"returnByValue": true,
		"awaitPromise":  true,
	})
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Result struct {
			Value json.RawMessage `json:"value"`
		} `json:"result"`
		ExceptionDetails *struct {
			Exception struct {
				Description string `json:"description"`
			} `json:"exception"`
		} `json:"exceptionDetails"`
	}
	if err := json.Unmarshal(result, &parsed); err != nil {
		return nil, fmt.Errorf("decode evaluate result: %w", err)
	}
	if parsed.ExceptionDetails != nil {
		description := parsed.ExceptionDetails.Exception.Description
		if idx := strings.IndexByte(description, '\n'); idx > 0 {
			description = description[:idx]
		}
		return nil, fmt.Errorf("page script failed: %s", description)
	}
	return parsed.Result.Value, nil
}
