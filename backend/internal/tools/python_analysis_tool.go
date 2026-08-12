package tools

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/sandbox"
)

// PythonAnalysisTool executes the existing restricted Python subset inside the
// shared OS sandbox Broker. The AST/builtin restrictions remain defense in
// depth; there is no host-Python fallback.
type PythonAnalysisTool struct {
	broker  *sandbox.Broker
	enabled bool
}

// NewPythonAnalysisTool accepts an optional Broker so bare registries remain
// source-compatible. Without a Broker the tool is disabled even when the legacy
// code-exec feature flag is set.
func NewPythonAnalysisTool(brokers ...*sandbox.Broker) *PythonAnalysisTool {
	var broker *sandbox.Broker
	if len(brokers) > 0 {
		broker = brokers[0]
	}
	return &PythonAnalysisTool{
		broker: broker,
		enabled: broker != nil && strings.EqualFold(
			strings.TrimSpace(os.Getenv("OMNILLM_CODE_EXEC_ENABLED")), "true",
		),
	}
}

func (t *PythonAnalysisTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:             "python_analysis",
		Description:      "Run restricted Python calculations and small in-memory data analysis inside the configured OS sandbox. Imports, file access, subprocesses, networking, and dynamic evaluation are blocked. Disabled unless explicitly enabled by the administrator and a sandbox Broker is available.",
		Category:         "compute",
		Enabled:          t != nil && t.enabled && t.broker != nil,
		Version:          "2",
		Risk:             RiskHigh,
		ReadOnly:         false,
		SideEffecting:    true,
		SupportsParallel: false,
		DefaultTimeoutMS: 10000,
		MaxResultBytes:   65536,
		Parameters: json.RawMessage(`{
			"type":"object",
			"required":["code"],
			"properties":{
				"code":{"type":"string","maxLength":20000,"description":"Restricted Python source. Assign the final serializable value to result or print output."},
				"data":{"description":"Optional JSON-serializable input exposed as variable data"}
			},
			"additionalProperties":false
		}`),
		OutputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"stdout":{"type":"string"},
				"result":{},
				"exit_code":{"type":"integer"}
			}
		}`),
		Examples: []ToolExample{
			{Description: "Calculate summary statistics", Arguments: json.RawMessage(`{"code":"values = data['values']\nresult = {'count': len(values), 'mean': statistics.mean(values), 'median': statistics.median(values)}","data":{"values":[4,8,15,16,23,42]}}`)},
		},
	}
}

type pythonAnalysisArgs struct {
	Code string          `json:"code"`
	Data json.RawMessage `json:"data"`
}

func (t *PythonAnalysisTool) Validate(raw json.RawMessage) error {
	if t == nil || !t.enabled || t.broker == nil {
		return fmt.Errorf("python analysis is disabled or sandbox unavailable")
	}
	var args pythonAnalysisArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return err
	}
	if strings.TrimSpace(args.Code) == "" {
		return fmt.Errorf("code is required")
	}
	if len(args.Code) > 20000 {
		return fmt.Errorf("code exceeds 20000 characters")
	}
	if len(args.Data) > 2*1024*1024 {
		return fmt.Errorf("data exceeds 2 MiB")
	}
	if len(args.Data) > 0 && !json.Valid(args.Data) {
		return fmt.Errorf("data must be valid JSON")
	}
	return nil
}

func (t *PythonAnalysisTool) Execute(ctx context.Context, raw json.RawMessage) (*ToolResult, error) {
	if t == nil || t.broker == nil || !t.enabled {
		return nil, fmt.Errorf("python analysis is disabled or sandbox unavailable")
	}
	var args pythonAnalysisArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	if len(args.Data) == 0 {
		args.Data = json.RawMessage(`null`)
	}
	owner, err := sandboxOwnerFromContext(ctx)
	if err != nil {
		return nil, err
	}

	spec := defaultCodeSandboxSpec(10000)
	spec.Resources.MaxStdoutBytes = 65536
	spec.Resources.MaxStderrBytes = 65536
	spec.Resources.MaxArtifactBytes = 0
	spec.TTLSeconds = 120
	session, err := t.broker.Create(ctx, owner, spec)
	if err != nil {
		return nil, err
	}

	program := restrictedPythonProgram(args.Code, args.Data)
	out, execErr := t.broker.Exec(ctx, owner, session.ID, sandbox.ExecRequest{
		Language:  "python",
		Code:      program,
		TimeoutMS: 10000,
	})
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	cleanupErr := t.broker.Destroy(cleanupCtx, owner, session.ID)
	cancel()
	if execErr != nil {
		return nil, execErr
	}
	if cleanupErr != nil {
		return nil, fmt.Errorf("restricted python sandbox cleanup failed: %w", cleanupErr)
	}
	if out == nil {
		return nil, fmt.Errorf("analysis returned no sandbox result")
	}
	if out.ExitCode != 0 {
		message := strings.TrimSpace(out.Stderr)
		if message == "" {
			message = fmt.Sprintf("sandbox exited with code %d", out.ExitCode)
		}
		return nil, fmt.Errorf("restricted python execution failed: %s", message)
	}

	output := bytes.TrimSpace([]byte(out.Stdout))
	if len(output) == 0 {
		return nil, fmt.Errorf("analysis returned no output")
	}
	var structured json.RawMessage
	if json.Valid(output) {
		structured = append(json.RawMessage(nil), output...)
	}
	content := string(output)
	if structured != nil {
		var envelope struct {
			Stdout string      `json:"stdout"`
			Result interface{} `json:"result"`
		}
		if json.Unmarshal(structured, &envelope) == nil {
			pretty, _ := json.MarshalIndent(envelope.Result, "", "  ")
			content = strings.TrimSpace(envelope.Stdout)
			if len(pretty) > 0 && string(pretty) != "null" {
				if content != "" {
					content += "\n"
				}
				content += string(pretty)
			}
		}
	}
	return &ToolResult{
		Content:    content,
		Structured: structured,
		Metadata: map[string]interface{}{
			"runtime":        t.broker.Capabilities().Name,
			"network":        "none",
			"workspace_mode": "ephemeral",
			"execution_id":   out.ExecutionID,
		},
	}, nil
}

func restrictedPythonProgram(code string, data json.RawMessage) string {
	encodedCode := base64.StdEncoding.EncodeToString([]byte(code))
	encodedData := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf(restrictedPythonWrapper, encodedCode, encodedData)
}

const restrictedPythonWrapper = `
import ast
import base64
import json
import math
import statistics
from io import StringIO

code = base64.b64decode(%q).decode("utf-8")
data = json.loads(base64.b64decode(%q).decode("utf-8"))

blocked_calls = {
    "eval", "exec", "compile", "open", "input", "globals", "locals", "vars",
    "getattr", "setattr", "delattr", "dir", "help", "memoryview", "breakpoint",
    "__import__"
}
blocked_names = {"os", "sys", "subprocess", "socket", "pathlib", "shutil", "ctypes", "builtins"}

tree = ast.parse(code, mode="exec")
for node in ast.walk(tree):
    if isinstance(node, (ast.Import, ast.ImportFrom, ast.Global, ast.Nonlocal)):
        raise ValueError("imports and global/nonlocal statements are not allowed")
    if isinstance(node, ast.Name) and (node.id in blocked_names or node.id.startswith("__")):
        raise ValueError("blocked name: " + node.id)
    if isinstance(node, ast.Attribute) and node.attr.startswith("_"):
        raise ValueError("private and dunder attributes are not allowed")
    if isinstance(node, ast.Call) and isinstance(node.func, ast.Name) and node.func.id in blocked_calls:
        raise ValueError("blocked call: " + node.func.id)

safe_builtins = {
    "abs": abs, "all": all, "any": any, "bool": bool, "dict": dict,
    "enumerate": enumerate, "filter": filter, "float": float, "int": int,
    "len": len, "list": list, "map": map, "max": max, "min": min,
    "print": print, "range": range, "reversed": reversed, "round": round,
    "set": set, "sorted": sorted, "str": str, "sum": sum, "tuple": tuple,
    "zip": zip
}
namespace = {
    "__builtins__": safe_builtins,
    "data": data,
    "json": json,
    "math": math,
    "statistics": statistics,
    "result": None,
}

capture = StringIO()
safe_builtins["print"] = lambda *args, **kwargs: print(*args, file=capture, **kwargs)
exec(compile(tree, "<analysis>", "exec"), namespace, namespace)
print(json.dumps({"stdout": capture.getvalue(), "result": namespace.get("result"), "exit_code": 0}, default=str))
`
