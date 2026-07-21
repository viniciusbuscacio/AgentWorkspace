package agent

import "testing"

func oaCall(name, args string) oaToolCall {
	var c oaToolCall
	c.ID = "call-1"
	c.Function.Name = name
	c.Function.Arguments = args
	return c
}

func TestToolCallPartsDecodesObjectArguments(t *testing.T) {
	parts, err := toolCallParts("", []oaToolCall{oaCall("aw", `{"action":"shell.exec","command":"echo hi"}`)})
	if err != nil {
		t.Fatalf("toolCallParts: %v", err)
	}
	fc := parts[0].FunctionCall
	if fc == nil || fc.Args["action"] != "shell.exec" || fc.Args["command"] != "echo hi" {
		t.Fatalf("unexpected args: %+v", parts[0])
	}
}

func TestToolCallPartsRecoversDoubleEncodedArguments(t *testing.T) {
	// A model that emits the arguments as a JSON STRING wrapping the object
	// (the "cannot unmarshal string into map" failure) must still be decoded.
	parts, err := toolCallParts("", []oaToolCall{oaCall("aw", `"{\"action\":\"shell.exec\",\"command\":\"echo hi\"}"`)})
	if err != nil {
		t.Fatalf("toolCallParts should recover double-encoded args: %v", err)
	}
	fc := parts[0].FunctionCall
	if fc == nil || fc.Args["action"] != "shell.exec" {
		t.Fatalf("unexpected recovered args: %+v", parts[0])
	}
}

func TestToolCallPartsRejectsBareString(t *testing.T) {
	// A bare non-JSON string has no recoverable structure and must still error.
	if _, err := toolCallParts("", []oaToolCall{oaCall("aw", `"echo hi"`)}); err == nil {
		t.Fatal("want error for a bare non-object string argument")
	}
}
