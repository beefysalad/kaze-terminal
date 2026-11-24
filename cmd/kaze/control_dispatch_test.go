package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// Exercise main's ordering without allowing a regression to open a real window.
func TestControlMainDispatchOrder(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var calls []string
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "main" {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			switch fun := call.Fun.(type) {
			case *ast.Ident:
				calls = append(calls, fun.Name)
			case *ast.SelectorExpr:
				if pkg, ok := fun.X.(*ast.Ident); ok {
					calls = append(calls, pkg.Name+"."+fun.Sel.Name)
				}
			}
			return true
		})
	}
	position := func(name string) int {
		for i, call := range calls {
			if call == name {
				return i
			}
		}
		t.Fatalf("missing %s call", name)
		return -1
	}
	codex, control, workspace, window := position("codexlaunch.Dispatch"), position("dispatchControl"), position("workspaceArg"), position("app.Main")
	if !(codex < control && control < workspace && workspace < window) {
		t.Fatalf("main dispatch order: %v", calls)
	}
}
