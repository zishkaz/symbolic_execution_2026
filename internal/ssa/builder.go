// Package ssa предоставляет функции для построения SSA представления
package ssa

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/ssa"
)

// Builder отвечает за построение SSA из исходного кода Go
type Builder struct {
	fset *token.FileSet
}

// NewBuilder создаёт новый экземпляр Builder
func NewBuilder() *Builder {
	return &Builder{
		fset: token.NewFileSet(),
	}
}

// TODO: Реализуйте следующие методы в рамках домашнего задания

// ParseAndBuildSSA парсит исходный код Go и создаёт SSA представление
// Возвращает SSA программу и функцию по имени
func (b *Builder) ParseAndBuildSSA(source string, funcName string) (*ssa.Function, error) {
	file, err := parser.ParseFile(b.fset, "main.go", source, 0)
	if err != nil {
		return nil, fmt.Errorf("Parsing error: %w", err)
	}

	conf := types.Config{Importer: importer.Default()}
	info := &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Defs:       make(map[*ast.Ident]types.Object),
		Uses:       make(map[*ast.Ident]types.Object),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
		Implicits:  make(map[ast.Node]types.Object),
		Scopes:     make(map[ast.Node]*types.Scope),
		Instances:  make(map[*ast.Ident]types.Instance),
	}
	pkg, err := conf.Check(file.Name.Name, b.fset, []*ast.File{file}, info)
	if err != nil {
		return nil, fmt.Errorf("Typing error: %w", err)
	}

	prog := ssa.NewProgram(b.fset, ssa.SanityCheckFunctions)
	ssapkg := prog.CreatePackage(pkg, []*ast.File{file}, info, true)
	ssapkg.Build()

	fn := ssapkg.Func(funcName)
	if fn == nil {
		return nil, fmt.Errorf("Function %s not found", funcName)
	}

	return fn, nil
}
