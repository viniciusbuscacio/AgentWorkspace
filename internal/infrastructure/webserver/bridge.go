package webserver

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
)

// bridge dispatches {method, args} requests onto the host *App by reflection,
// mirroring exactly how the Wails IPC binds and calls App methods: each arg is
// JSON-unmarshaled into the real Go parameter type, and results are returned
// using the Wails convention (trailing error becomes {"error":...}, otherwise
// 0 results -> null, 1 -> the value, N -> an array).
type bridge struct {
	methods map[string]reflect.Value
}

// errorType is the reflect.Type of the error interface, used to detect a
// trailing error return value.
var errorType = reflect.TypeOf((*error)(nil)).Elem()

// contextType is the reflect.Type of context.Context. No *App method currently
// takes one, but if that changes we inject context.Background() so the bridge
// stays robust rather than failing an arg-count check.
var contextType = reflect.TypeOf((*context.Context)(nil)).Elem()

// newBridge builds the method table once from the bound value (reflect.ValueOf
// on the *App pointer). The map is read-only afterwards, so concurrent dispatch
// is safe without locking.
func newBridge(app any) *bridge {
	methods := map[string]reflect.Value{}
	v := reflect.ValueOf(app)
	t := v.Type()
	for i := 0; i < t.NumMethod(); i++ {
		methods[t.Method(i).Name] = v.Method(i)
	}
	return &bridge{methods: methods}
}

// bridgeRequest is the wire format the web shim posts to /bridge.
type bridgeRequest struct {
	Method string            `json:"method"`
	Args   []json.RawMessage `json:"args"`
}

// dispatchResult carries the marshaled body and an app-level error (a non-nil
// trailing error return), distinct from a protocol error (bad method/args).
type dispatchResult struct {
	body   any
	appErr string
	badReq string // non-empty -> HTTP 400 with this message
	denied bool
}

// dispatch runs one bridge request and returns the result to encode.
func (b *bridge) dispatch(req bridgeRequest) (res dispatchResult) {
	if req.Method == "" {
		return dispatchResult{badReq: "method is required"}
	}
	if IsDenied(req.Method) {
		// Shape mirrors pip-bindings' notAvailable so existing UI error
		// handling treats it uniformly.
		return dispatchResult{denied: true, body: map[string]any{
			"success": false,
			"error":   "not available in web mode",
		}}
	}
	method, ok := b.methods[req.Method]
	if !ok {
		return dispatchResult{badReq: fmt.Sprintf("unknown method %q", req.Method)}
	}
	mt := method.Type()

	// Build call arguments, injecting context.Background() for any
	// context.Context parameter and unmarshaling the rest from req.Args.
	in := make([]reflect.Value, 0, mt.NumIn())
	argIdx := 0
	for i := 0; i < mt.NumIn(); i++ {
		paramType := mt.In(i)
		if paramType == contextType {
			in = append(in, reflect.ValueOf(context.Background()))
			continue
		}
		if mt.IsVariadic() && i == mt.NumIn()-1 {
			// Defensive: no current method is variadic. Spread remaining args
			// into the slice's element type.
			elemType := paramType.Elem()
			for ; argIdx < len(req.Args); argIdx++ {
				val, err := unmarshalArg(elemType, req.Args[argIdx])
				if err != nil {
					return dispatchResult{badReq: err.Error()}
				}
				in = append(in, val)
			}
			break
		}
		if argIdx >= len(req.Args) {
			return dispatchResult{badReq: fmt.Sprintf(
				"method %s expects %d args, got %d", req.Method, countNonCtxParams(mt), len(req.Args))}
		}
		val, err := unmarshalArg(paramType, req.Args[argIdx])
		if err != nil {
			return dispatchResult{badReq: err.Error()}
		}
		in = append(in, val)
		argIdx++
	}
	if !mt.IsVariadic() && argIdx != len(req.Args) {
		return dispatchResult{badReq: fmt.Sprintf(
			"method %s expects %d args, got %d", req.Method, countNonCtxParams(mt), len(req.Args))}
	}

	// A panic inside the call becomes an HTTP 500 (handled by the caller), never
	// a crashed server.
	out := method.Call(in)
	return marshalResults(out)
}

// unmarshalArg allocates a value of paramType and JSON-unmarshals raw into it,
// returning the dereferenced value ready to pass to Call.
func unmarshalArg(paramType reflect.Type, raw json.RawMessage) (reflect.Value, error) {
	ptr := reflect.New(paramType)
	if len(raw) == 0 {
		return ptr.Elem(), nil
	}
	if err := json.Unmarshal(raw, ptr.Interface()); err != nil {
		return reflect.Value{}, fmt.Errorf("invalid argument for %s: %w", paramType.String(), err)
	}
	return ptr.Elem(), nil
}

// marshalResults applies the Wails return convention.
func marshalResults(out []reflect.Value) dispatchResult {
	// Trailing error.
	if n := len(out); n > 0 && out[n-1].Type() == errorType {
		if errVal := out[n-1]; !errVal.IsNil() {
			if err, ok := errVal.Interface().(error); ok {
				return dispatchResult{appErr: err.Error()}
			}
		}
		out = out[:n-1]
	}
	switch len(out) {
	case 0:
		return dispatchResult{body: nil}
	case 1:
		return dispatchResult{body: out[0].Interface()}
	default:
		values := make([]any, len(out))
		for i, v := range out {
			values[i] = v.Interface()
		}
		return dispatchResult{body: values}
	}
}

func countNonCtxParams(mt reflect.Type) int {
	n := 0
	for i := 0; i < mt.NumIn(); i++ {
		if mt.In(i) != contextType {
			n++
		}
	}
	return n
}
