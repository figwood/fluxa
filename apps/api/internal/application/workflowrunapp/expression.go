package workflowrunapp

import (
	"fmt"
	"go/ast"
	"go/parser"
	"strconv"
	"strings"
	"time"
)

// expressionProgram implements the deliberately small, side-effect-free CEL
// subset used by workflow definitions. It accepts selectors, literals,
// comparisons, &&/||/!, and the hasService/inTeam helpers.
type expressionProgram struct{ expr ast.Expr }

func compileBool(expression string) (*expressionProgram, error) {
	expr, err := parser.ParseExpr(strings.TrimSpace(expression))
	if err != nil {
		return nil, err
	}
	return &expressionProgram{expr: expr}, nil
}

func (p *expressionProgram) Eval(vars map[string]any) (bool, error) {
	v, err := evalExpression(p.expr, vars)
	if err != nil {
		return false, err
	}
	b, ok := v.(bool)
	if !ok {
		return false, fmt.Errorf("expression must return bool")
	}
	return b, nil
}

func evalExpression(expr ast.Expr, vars map[string]any) (any, error) {
	switch n := expr.(type) {
	case *ast.ParenExpr:
		return evalExpression(n.X, vars)
	case *ast.Ident:
		if n.Name == "true" {
			return true, nil
		}
		if n.Name == "false" {
			return false, nil
		}
		v, ok := vars[n.Name]
		if !ok {
			return nil, fmt.Errorf("unknown variable %s", n.Name)
		}
		return v, nil
	case *ast.BasicLit:
		switch n.Kind.String() {
		case "STRING":
			return strconv.Unquote(n.Value)
		case "INT":
			return strconv.ParseInt(n.Value, 10, 64)
		case "FLOAT":
			return strconv.ParseFloat(n.Value, 64)
		}
	case *ast.SelectorExpr:
		base, err := evalExpression(n.X, vars)
		if err != nil {
			return nil, err
		}
		m, ok := base.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("cannot select %s", n.Sel.Name)
		}
		return m[n.Sel.Name], nil
	case *ast.UnaryExpr:
		v, err := evalExpression(n.X, vars)
		if err != nil {
			return nil, err
		}
		if n.Op.String() == "!" {
			b, ok := v.(bool)
			if !ok {
				return nil, fmt.Errorf("! requires bool")
			}
			return !b, nil
		}
	case *ast.BinaryExpr:
		left, err := evalExpression(n.X, vars)
		if err != nil {
			return nil, err
		}
		if n.Op.String() == "&&" {
			b, ok := left.(bool)
			if !ok {
				return nil, fmt.Errorf("&& requires bool")
			}
			if !b {
				return false, nil
			}
		}
		if n.Op.String() == "||" {
			b, ok := left.(bool)
			if !ok {
				return nil, fmt.Errorf("|| requires bool")
			}
			if b {
				return true, nil
			}
		}
		right, err := evalExpression(n.Y, vars)
		if err != nil {
			return nil, err
		}
		switch n.Op.String() {
		case "&&":
			return right, nil
		case "||":
			return right, nil
		case "==":
			return fmt.Sprint(left) == fmt.Sprint(right), nil
		case "!=":
			return fmt.Sprint(left) != fmt.Sprint(right), nil
		case ">", ">=", "<", "<=":
			return compareValues(left, right, n.Op.String())
		}
	case *ast.CallExpr:
		fn, ok := n.Fun.(*ast.Ident)
		if !ok {
			return nil, fmt.Errorf("unsupported call")
		}
		args := make([]any, 0, len(n.Args))
		for _, arg := range n.Args {
			v, e := evalExpression(arg, vars)
			if e != nil {
				return nil, e
			}
			args = append(args, v)
		}
		switch fn.Name {
		case "hasService":
			if len(args) != 1 {
				return nil, fmt.Errorf("hasService requires one argument")
			}
			release, _ := vars["release"].(map[string]any)
			switch services := release["services"].(type) {
			case []map[string]any:
				for _, svc := range services {
					if fmt.Sprint(svc["key"]) == fmt.Sprint(args[0]) {
						return true, nil
					}
				}
			case []any:
				for _, raw := range services {
					if svc, ok := raw.(map[string]any); ok && fmt.Sprint(svc["key"]) == fmt.Sprint(args[0]) {
						return true, nil
					}
				}
			}
			return false, nil
		case "inTeam":
			if len(args) != 1 {
				return nil, fmt.Errorf("inTeam requires one argument")
			}
			project, _ := vars["project"].(map[string]any)
			return fmt.Sprint(project["team"]) == fmt.Sprint(args[0]), nil
		}
	}
	return nil, fmt.Errorf("unsupported expression")
}

func compareValues(a, b any, op string) (bool, error) {
	af, ae := strconv.ParseFloat(fmt.Sprint(a), 64)
	bf, be := strconv.ParseFloat(fmt.Sprint(b), 64)
	if ae != nil || be != nil {
		return false, fmt.Errorf("comparison requires numbers")
	}
	switch op {
	case ">":
		return af > bf, nil
	case ">=":
		return af >= bf, nil
	case "<":
		return af < bf, nil
	default:
		return af <= bf, nil
	}
}

func evalTimestamp(expression string, vars map[string]any) (time.Time, error) {
	raw := strings.TrimSpace(expression)
	if strings.HasPrefix(raw, "\"") {
		value, err := strconv.Unquote(raw)
		if err != nil {
			return time.Time{}, err
		}
		return time.Parse(time.RFC3339, value)
	}
	parts := strings.Split(raw, ".")
	var value any = vars
	for _, part := range parts {
		m, ok := value.(map[string]any)
		if !ok {
			return time.Time{}, fmt.Errorf("invalid timestamp selector")
		}
		value = m[part]
	}
	switch v := value.(type) {
	case time.Time:
		return v, nil
	case string:
		return time.Parse(time.RFC3339, v)
	default:
		return time.Time{}, fmt.Errorf("timestamp expression must resolve to RFC3339 string")
	}
}
