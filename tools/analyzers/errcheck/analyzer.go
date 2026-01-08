// Package errcheck implements an static analysis analyzer to ensure that errors are handled in go
// code. This analyzer was adapted from https://github.com/kisielk/errcheck (MIT License).
package errcheck

import (
	"go/token"

	errcheck "github.com/kisielk/errcheck/errcheck"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/packages"
)

const Doc = "Check for unchecked errors"

var Analyzer = &analysis.Analyzer{
	Name: "errcheck",
	Doc:  Doc,
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	checker := errcheck.Checker{}
	pkg := &packages.Package{
		Fset:      pass.Fset,
		Syntax:    pass.Files,
		TypesInfo: pass.TypesInfo,
	}
	result := checker.CheckPackage(pkg)
	for _, unchecked := range result.UncheckedErrors {
		pos := positionToPos(pass.Fset, unchecked.Pos)
		message := unchecked.Line
		if message == "" {
			message = "unchecked error"
		}
		pass.Reportf(pos, "unchecked error: %s", message)
	}
	return nil, nil
}

func positionToPos(fset *token.FileSet, position token.Position) token.Pos {
	if position.Filename == "" {
		return token.NoPos
	}

	var file *token.File
	fset.Iterate(func(f *token.File) bool {
		if f.Name() == position.Filename {
			file = f
			return false
		}
		return true
	})
	if file == nil {
		return token.NoPos
	}
	if position.Offset >= 0 && position.Offset <= file.Size() {
		return file.Pos(position.Offset)
	}
	return token.NoPos
}
