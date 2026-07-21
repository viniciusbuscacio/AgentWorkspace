package tools

import "context"

// Dispatcher exposes the aw action registry to transport-agnostic callers —
// today the MCP server — without going through the ADK tool layer. It shares
// the workspace semantics (confirmation flow, path sandboxing, feature gates)
// with the agent's aw tool.
type Dispatcher struct {
	ws          *workspace
	description string
}

// NewDispatcher builds a dispatcher over the same action registry the agent's
// aw tool uses. Returns (nil, nil) when no workspace root is configured,
// mirroring New.
func NewDispatcher(opts Options) (*Dispatcher, error) {
	ws, err := newWorkspace(opts)
	if err != nil || ws == nil {
		return nil, err
	}
	return &Dispatcher{ws: ws, description: awToolDescription(opts)}, nil
}

// Call runs one aw action. argsJSON carries the action's arguments as a JSON
// object string (empty for none) and the return value is the action's JSON
// output.
func (d *Dispatcher) Call(ctx context.Context, action string, argsJSON string) (string, error) {
	result, err := d.ws.dispatchAction(contextOrBackground(ctx), awArgs{Action: action, Args: argsJSON})
	if err != nil {
		return "", err
	}
	return result.Result, nil
}

// Description documents the aw tool for the enabled action groups, in the
// same wording the ADK tool uses.
func (d *Dispatcher) Description() string {
	return d.description
}
