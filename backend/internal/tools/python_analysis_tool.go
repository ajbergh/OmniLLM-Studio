package tools

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// PythonAnalysisTool executes a deliberately restricted Python subset in an
// isolated temporary working directory. It is disabled unless
// OMNILLM_CODE_EXEC_ENABLED=true and should retain an "ask" policy.
//
// This is not a replacement for an OS/container sandbox. It intentionally
// excludes imports, filesystem APIs, process APIs, network modules, dunder
// access, and dynamic evaluation. A future sandbox package can replace the
// runner without changing the tool contract.
type PythonAnalysisTool struct {
	pythonPath string
	enabled    bool
}

func NewPythonAnalysisTool() *PythonAnalysisTool {
	pythonPath := strings.TrimSpace(os.Getenv("OMNILLM_PYTHON_EXEC"))
	if pythonPath == "" {
		pythonPath = "python3"
		if runtime.GOOS == "windows" {
			pythonPath = "python"
		}
	}
	return &PythonAnalysisTool{
		pythonPath: pythonPath,
		enabled:    strings.EqualFold(strings.TrimSpace(os.Getenv("OMNILLM_CODE_EXEC_ENABLED")), "true"),
	}
}

func (t *PythonAnalysisTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:             "python_analysis",
		Description:      "Run restricted Python calculations and small in-memory data analysis. Imports, file access, subprocesses, networking, and dynamic evaluation are blocked. Disabled unless explicitly enabled by the administrator.",
		Category:         "compute",
		Enabled:          t.enabled,
		Version:          "1",
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
			}
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
	if !t.enabled {
		return fmt.Errorf("python analysis is disabled")
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
	var args pythonAnalysisArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	if len(args.Data) == 0 {
		args.Data = json.RawMessage(`null`)
	}

	workDir, err := os.MkdirTemp("", "omnillm-python-analysis-*")
	if err != nil {
		return nil, fmt.Errorf("create analysis workspace: %w", err)
	}
	defer os.RemoveAll(workDir)

	wrapperPath := filepath.Join(workDir, "runner.py")
	if err := os.WriteFile(wrapperPath, []byte(restrictedPythonWrapper), 0o600); err != nil {
		return nil, fmt.Errorf("write analysis runner: %w", err)
	}

	encodedCode := base64.StdEncoding.EncodeToString([]byte(args.Code))
	encodedData := base64.StdEncoding.EncodeToString(args.Data)
	cmd := exec.CommandContext(ctx, t.pythonPath, "-I", "-S", wrapperPath)
	cmd.Dir = workDir
	cmd.Env = []string{
		"PYTHONIOENCODING=utf-8",
		"OMNILLM_ANALYSIS_CODE=" + encodedCode,
		"OMNILLM_ANALYSIS_DATA=" + encodedData,
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("restricted python execution failed: %s", message)
	}

	output := bytes.TrimSpace(stdout.Bytes())
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
			"runtime":       "restricted-python",
			"network":       "not exposed by contract",
			"workspace_mode": "temporary",
		},
	}, nil
}

const restrictedPythonWrapper = `
import ast
import base64
import json
import math
import os
import statistics

code = base64.b64decode(os.environ["OMNILLM_ANALYSIS_CODE"]).decode("utf-8")
data = json.loads(base64.b64decode(os.environ["OMNILLM_ANALYSIS_DATA"]).decode("utf-8"))

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

from io import StringIO
capture = StringIO()
safe_builtins["print"] = lambda *args, **kwargs: print(*args, file=capture, **kwargs)
exec(compile(tree, "<analysis>", "exec"), namespace, namespace)
print(json.dumps({"stdout": capture.getvalue(), "result": namespace.get("result"), "exit_code": 0}, default=str))
`
