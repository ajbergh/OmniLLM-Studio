package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"math"
	"strconv"
)

// CalculatorTool evaluates simple arithmetic expressions.
type CalculatorTool struct{}

func NewCalculatorTool() *CalculatorTool { return &CalculatorTool{} }

type calculatorArgs struct {
	Expression string `json:"expression"`
}

func (t *CalculatorTool) Definition() ToolDefinition {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"expression": {
				"type": "string",
				"description": "A mathematical expression to evaluate, e.g. '(2+3)*4'"
			}
		},
		"required": ["expression"]
	}`)

	return ToolDefinition{
		Name:        "calculator",
		Description: "Evaluate a mathematical expression and return the numeric result.",
		Parameters:  schema,
		Category:    "compute",
		Enabled:     true,
	}
}

func (t *CalculatorTool) Validate(args json.RawMessage) error {
	var a calculatorArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if a.Expression == "" {
		return fmt.Errorf("expression is required")
	}
	return nil
}

func (t *CalculatorTool) Execute(_ context.Context, args json.RawMessage) (*ToolResult, error) {
	var a calculatorArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("unmarshal args: %w", err)
	}

	result, err := evalExpr(a.Expression)
	if err != nil {
		return &ToolResult{
			Content: fmt.Sprintf("Error evaluating expression: %v", err),
			IsError: true,
		}, nil
	}

	// Format nicely: drop trailing .0 for integers
	formatted := strconv.FormatFloat(result, 'f', -1, 64)

	return &ToolResult{
		Content: formatted,
		Metadata: map[string]interface{}{
			"expression": a.Expression,
			"result":     result,
		},
	}, nil
}

// ---------------------------------------------------------------------------
// Safe expression evaluator using Go's AST parser (no eval/exec)
// ---------------------------------------------------------------------------

func evalExpr(expr string) (float64, error) {
	node, err := parser.ParseExpr(expr)
	if err != nil {
		return 0, fmt.Errorf("parse error: %w", err)
	}
	return evalNode(node)
}

func evalNode(node ast.Expr) (float64, error) {
	switch n := node.(type) {
	case *ast.BasicLit:
		if n.Kind != token.INT && n.Kind != token.FLOAT {
			return 0, fmt.Errorf("unsupported literal: %s", n.Value)
		}
		return strconv.ParseFloat(n.Value, 64)

	case *ast.ParenExpr:
		return evalNode(n.X)

	case *ast.UnaryExpr:
		x, err := evalNode(n.X)
		if err != nil {
			return 0, err
		}
		switch n.Op {
		case token.SUB:
			return -x, nil
		case token.ADD:
			return x, nil
		default:
			return 0, fmt.Errorf("unsupported unary operator: %s", n.Op)
		}

	case *ast.BinaryExpr:
		left, err := evalNode(n.X)
		if err != nil {
			return 0, err
		}
		right, err := evalNode(n.Y)
		if err != nil {
			return 0, err
		}
		switch n.Op {
		case token.ADD:
			return left + right, nil
		case token.SUB:
			return left - right, nil
		case token.MUL:
			return left * right, nil
		case token.QUO:
			if right == 0 {
				return 0, fmt.Errorf("division by zero")
			}
			return left / right, nil
		case token.REM:
			if right == 0 {
				return 0, fmt.Errorf("modulo by zero")
			}
			return math.Mod(left, right), nil
		default:
			return 0, fmt.Errorf("unsupported operator: %s", n.Op)
		}

	default:
		return 0, fmt.Errorf("unsupported expression type: %T", node)
	}
}
