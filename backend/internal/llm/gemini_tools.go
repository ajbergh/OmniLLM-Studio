package llm

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// Gemini native grounding and OpenAI-shaped function calling in one request.
//
// The grounded Gemini adapter originally emitted only `google_search` and threw
// away any function declarations, which made grounding and tool calling mutually
// exclusive: a grounded turn could not call a tool, and a tool-calling turn could
// not be grounded. Everything in this file exists to remove that exclusion.
//
// Three conversions are required, and omitting any one of them silently breaks a
// multi-round tool loop:
//
//  1. Requests: OpenAI `tools` -> Gemini `function_declarations`.
//  2. History: an assistant turn carrying tool calls, and the `role: "tool"`
//     results that answer it, must round-trip as Gemini `functionCall` and
//     `functionResponse` parts. Without this, round two of a loop resends no
//     evidence that the tool ever ran and the model calls it again forever.
//  3. Responses: Gemini `functionCall` parts -> OpenAI `tool_calls`, with a
//     `tool_calls` finish reason so the existing loop notices them.

// geminiFunctionCallIDPrefix marks synthesized tool-call IDs. Gemini does not
// assign IDs to function calls, but the OpenAI shape requires them to correlate
// results, so they are generated here and matched back by function name.
const geminiFunctionCallIDPrefix = "gemini_fc_"

// newGeminiToolCallID mints an ID for a Gemini function call.
func newGeminiToolCallID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		// A collision only affects correlation inside a single response, and the
		// name-based fallback in geminiFunctionResponseName covers it.
		return geminiFunctionCallIDPrefix + "0"
	}
	return geminiFunctionCallIDPrefix + hex.EncodeToString(buf)
}

// geminiFunctionDeclarations converts OpenAI tool definitions to Gemini's shape.
//
// Gemini rejects an empty `parameters` object on some model families, so a tool
// with no schema is emitted without the field rather than with `{}`.
func geminiFunctionDeclarations(rawTools []interface{}) []map[string]interface{} {
	declarations := make([]map[string]interface{}, 0, len(rawTools))
	for _, raw := range rawTools {
		tool, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		// Skip anything that is not a plain function tool: provider-native server
		// tools are added separately and must not be re-declared as functions.
		if toolType, _ := tool["type"].(string); toolType != "" && toolType != "function" {
			continue
		}
		function, ok := tool["function"].(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := function["name"].(string)
		if strings.TrimSpace(name) == "" {
			continue
		}
		declaration := map[string]interface{}{"name": name}
		if description, _ := function["description"].(string); description != "" {
			declaration["description"] = description
		}
		if parameters := sanitizeGeminiSchema(function["parameters"]); parameters != nil {
			declaration["parameters"] = parameters
		}
		declarations = append(declarations, declaration)
	}
	if len(declarations) == 0 {
		return nil
	}
	return declarations
}

// geminiUnsupportedSchemaKeys are JSON Schema keywords Gemini's function
// declaration schema does not accept. Sending them is a 400, so they are dropped
// rather than passed through.
var geminiUnsupportedSchemaKeys = map[string]bool{
	"$schema":              true,
	"$id":                  true,
	"additionalProperties": true,
	"definitions":          true,
	"$defs":                true,
	"examples":             true,
	"default":              true,
	"const":                true,
	"oneOf":                true,
	"allOf":                true,
	"not":                  true,
	"exclusiveMinimum":     true,
	"exclusiveMaximum":     true,
	"patternProperties":    true,
	"readOnly":             true,
	"writeOnly":            true,
	"title":                true,
}

// sanitizeGeminiSchema recursively strips keywords Gemini rejects.
//
// Tool schemas in this repository are hand-written JSON Schema aimed at OpenAI,
// which accepts a much larger keyword set. Passing one through unchanged is a
// 400 from Gemini, and a 400 on a grounded request degrades to local search
// rather than surfacing — so the schema is narrowed here instead.
func sanitizeGeminiSchema(value interface{}) interface{} {
	switch typed := value.(type) {
	case json.RawMessage:
		var decoded interface{}
		if json.Unmarshal(typed, &decoded) != nil {
			return nil
		}
		return sanitizeGeminiSchema(decoded)
	case []byte:
		var decoded interface{}
		if json.Unmarshal(typed, &decoded) != nil {
			return nil
		}
		return sanitizeGeminiSchema(decoded)
	case map[string]interface{}:
		out := make(map[string]interface{}, len(typed))
		for key, child := range typed {
			if geminiUnsupportedSchemaKeys[key] {
				continue
			}
			if sanitized := sanitizeGeminiSchema(child); sanitized != nil {
				out[key] = sanitized
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	case []interface{}:
		out := make([]interface{}, 0, len(typed))
		for _, child := range typed {
			if sanitized := sanitizeGeminiSchema(child); sanitized != nil {
				out = append(out, sanitized)
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	default:
		return value
	}
}

// geminiToolCallParts converts an assistant message's tool calls to Gemini
// functionCall parts.
func geminiToolCallParts(rawCalls []interface{}) []map[string]interface{} {
	parts := make([]map[string]interface{}, 0, len(rawCalls))
	for _, raw := range rawCalls {
		call, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		function, ok := call["function"].(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := function["name"].(string)
		if strings.TrimSpace(name) == "" {
			continue
		}
		args := map[string]interface{}{}
		if encoded, _ := function["arguments"].(string); strings.TrimSpace(encoded) != "" {
			// A malformed argument string must not drop the call: Gemini needs the
			// functionCall part present to pair with the functionResponse that
			// follows, or the conversation is invalid.
			_ = json.Unmarshal([]byte(encoded), &args)
		}
		parts = append(parts, map[string]interface{}{
			"functionCall": map[string]interface{}{"name": name, "args": args},
		})
	}
	if len(parts) == 0 {
		return nil
	}
	return parts
}

// geminiFunctionResponsePart converts a `role: "tool"` message to a Gemini
// functionResponse part.
//
// Gemini keys function responses by function *name*, not by call ID, so the name
// must be recovered — from the message's own name field when present, otherwise
// from the tool call it answers.
func geminiFunctionResponsePart(name, content string) map[string]interface{} {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "tool"
	}
	// Gemini requires an object response. Tool output here is free-form text, so
	// it is wrapped rather than parsed: a result that happens to be valid JSON is
	// passed through as structured data, anything else travels as a string.
	response := map[string]interface{}{}
	trimmed := strings.TrimSpace(content)
	if strings.HasPrefix(trimmed, "{") {
		var decoded map[string]interface{}
		if json.Unmarshal([]byte(trimmed), &decoded) == nil {
			response = decoded
		}
	}
	if len(response) == 0 {
		response = map[string]interface{}{"result": content}
	}
	return map[string]interface{}{
		"functionResponse": map[string]interface{}{
			"name":     name,
			"response": response,
		},
	}
}

// geminiFunctionCallName resolves the function name a tool result answers, by
// matching its tool_call_id against the assistant turn that requested it.
func geminiFunctionCallName(messages []interface{}, toolCallID string) string {
	toolCallID = strings.TrimSpace(toolCallID)
	if toolCallID == "" {
		return ""
	}
	for _, raw := range messages {
		message, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		calls, ok := message["tool_calls"].([]interface{})
		if !ok {
			continue
		}
		for _, rawCall := range calls {
			call, ok := rawCall.(map[string]interface{})
			if !ok {
				continue
			}
			if id, _ := call["id"].(string); id != toolCallID {
				continue
			}
			if function, ok := call["function"].(map[string]interface{}); ok {
				if name, _ := function["name"].(string); name != "" {
					return name
				}
			}
		}
	}
	return ""
}

// geminiFunctionCallsFromParts extracts OpenAI-shaped tool calls from Gemini
// response parts.
func geminiFunctionCallsFromParts(parts []geminiResponsePart) []interface{} {
	calls := make([]interface{}, 0, len(parts))
	for _, part := range parts {
		if part.FunctionCall == nil {
			continue
		}
		name := strings.TrimSpace(part.FunctionCall.Name)
		if name == "" {
			continue
		}
		arguments := "{}"
		if len(part.FunctionCall.Args) > 0 {
			arguments = string(part.FunctionCall.Args)
		}
		calls = append(calls, map[string]interface{}{
			"index": len(calls),
			"id":    newGeminiToolCallID(),
			"type":  "function",
			"function": map[string]interface{}{
				"name":      name,
				"arguments": arguments,
			},
		})
	}
	if len(calls) == 0 {
		return nil
	}
	return calls
}

// geminiToolConfig returns the function-calling mode.
//
// AUTO is deliberate: the model must be free to answer from grounding alone
// without calling a function, which is the common case on a grounded turn.
func geminiToolConfig() map[string]interface{} {
	return map[string]interface{}{
		"function_calling_config": map[string]interface{}{"mode": "AUTO"},
	}
}

// describeGeminiTools summarizes an outgoing tool set for logs.
func describeGeminiTools(grounding bool, declarations int) string {
	return fmt.Sprintf("grounding=%v function_declarations=%d", grounding, declarations)
}
