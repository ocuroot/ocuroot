package sdk

import (
	"fmt"
	"sort"

	"go.starlark.net/syntax"
)

type CallStackTree struct {
	FunctionName string          `json:"function_name"`
	Pos          syntax.Position `json:"pos"`

	def *syntax.DefStmt `json:"-"`

	Calls []Call `json:"calls"`
}

type Call struct {
	FunctionName string          `json:"function_name"`
	Filename     string          `json:"filename"`
	Pos          syntax.Position `json:"pos"`

	Params []Param `json:"params"`
}

type Param struct {
	Index  int     `json:"index"`
	Name   string  `json:"name"`
	Ident  *string `json:"ident,omitempty"`
	String *string `json:"string,omitempty"`
}

func BuildStackTrees(program *syntax.File) []*CallStackTree {
	trees := map[string]*CallStackTree{}

	for _, stmt := range program.Stmts {
		switch stmt := stmt.(type) {
		case *syntax.DefStmt:
			tree := &CallStackTree{
				FunctionName: stmt.Name.Name,
				Pos:          stmt.Def,
				def:          stmt,
			}
			trees[stmt.Name.Name] = tree
		}
	}

	for _, tree := range trees {
		tree.Calls = buildCallsFromStatements(tree.def.Body)
	}

	var out []*CallStackTree
	for _, tree := range trees {
		out = append(out, tree)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].FunctionName < out[j].FunctionName
	})

	return out
}

func (t *CallStackTree) String() string {
	var callNames []string
	for _, call := range t.Calls {
		callNames = append(callNames, call.FunctionName)
	}

	return fmt.Sprintf("%s %d %s", t.FunctionName, len(t.Calls), callNames)
}

func buildCallsFromStatements(stmts []syntax.Stmt) []Call {
	var calls []Call
	for _, stmt := range stmts {
		switch stmt := stmt.(type) {
		case *syntax.DefStmt:
			// This assumes that this function is called at some point
			calls = append(calls, buildCallsFromStatements(stmt.Body)...)
		case *syntax.ExprStmt:
			calls = append(calls, buildCallsFromExpression(stmt.X)...)
		case *syntax.IfStmt:
			calls = append(calls, buildCallsFromStatements(stmt.True)...)
			calls = append(calls, buildCallsFromStatements(stmt.False)...)
		case *syntax.ForStmt:
			calls = append(calls, buildCallsFromStatements(stmt.Body)...)
		case *syntax.AssignStmt:
			calls = append(calls, buildCallsFromExpression(stmt.RHS)...)
		case *syntax.BranchStmt:
		case *syntax.LoadStmt:
		case *syntax.ReturnStmt:
			calls = append(calls, buildCallsFromExpression(stmt.Result)...)
		case *syntax.WhileStmt:
			calls = append(calls, buildCallsFromStatements(stmt.Body)...)
		}
	}
	return calls
}

func callFromIdent(ident *syntax.Ident) Call {
	return Call{
		FunctionName: ident.Name,
		Pos:          ident.NamePos,
		Filename:     ident.NamePos.Filename(),
	}
}

func exprName(expr syntax.Expr) string {
	switch expr := expr.(type) {
	case *syntax.Ident:
		return expr.Name
	case *syntax.DotExpr:
		return fmt.Sprintf("%s.%s", exprName(expr.X), expr.Name.Name)
	}
	return ""
}

func paramFromExpr(expr syntax.Expr) Param {
	switch expr := expr.(type) {
	case *syntax.Ident:
		return Param{Ident: &expr.Name}
	case *syntax.Literal:
		s := fmt.Sprint(expr.Value)
		return Param{String: &s}
	case *syntax.BinaryExpr:
		p := paramFromExpr(expr.Y)
		p.Name = exprName(expr.X)
		return p
	}
	return Param{}
}

func callFromCallExpr(expr *syntax.CallExpr) Call {
	var c Call
	if dot, ok := expr.Fn.(*syntax.DotExpr); ok {
		c = callFromIdent(dot.Name)
		c.FunctionName = exprName(dot)
	}
	if i, ok := expr.Fn.(*syntax.Ident); ok {
		c = callFromIdent(i)
		c.FunctionName = exprName(i)
	}

	for i, arg := range expr.Args {
		c.Params = append(c.Params, paramFromExpr(arg))
		c.Params[i].Index = i
	}

	return c
}

func buildCallsFromExpression(expr syntax.Expr) []Call {
	var calls []Call
	switch expr := expr.(type) {
	case *syntax.BinaryExpr:
		calls = append(calls, buildCallsFromExpression(expr.X)...)
		calls = append(calls, buildCallsFromExpression(expr.Y)...)
	case *syntax.CallExpr:
		calls = append(calls, callFromCallExpr(expr))
	case *syntax.Comprehension:
		calls = append(calls, buildCallsFromExpression(expr.Body)...)
		// TODO: Handle the clauses
	case *syntax.CondExpr:
		calls = append(calls, buildCallsFromExpression(expr.True)...)
		calls = append(calls, buildCallsFromExpression(expr.False)...)
	case *syntax.DictEntry:
		calls = append(calls, buildCallsFromExpression(expr.Key)...)
		calls = append(calls, buildCallsFromExpression(expr.Value)...)
	case *syntax.DictExpr:
		for _, entry := range expr.List {
			calls = append(calls, buildCallsFromExpression(entry)...)
		}
	case *syntax.DotExpr:
		calls = append(calls, buildCallsFromExpression(expr.X)...)
	case *syntax.Ident:
	case *syntax.IndexExpr:
		calls = append(calls, buildCallsFromExpression(expr.X)...)
	case *syntax.LambdaExpr:
	case *syntax.ListExpr:
		for _, expr := range expr.List {
			calls = append(calls, buildCallsFromExpression(expr)...)
		}
	case *syntax.Literal:
	case *syntax.ParenExpr:
		calls = append(calls, buildCallsFromExpression(expr.X)...)
	case *syntax.SliceExpr:
		calls = append(calls, buildCallsFromExpression(expr.X)...)
		calls = append(calls, buildCallsFromExpression(expr.Lo)...)
		calls = append(calls, buildCallsFromExpression(expr.Hi)...)
		calls = append(calls, buildCallsFromExpression(expr.Step)...)
	case *syntax.TupleExpr:
		for _, expr := range expr.List {
			calls = append(calls, buildCallsFromExpression(expr)...)
		}
	case *syntax.UnaryExpr:
		calls = append(calls, buildCallsFromExpression(expr.X)...)
	}
	return calls
}
