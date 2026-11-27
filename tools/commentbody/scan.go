package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"
)

const docCap = 10

const (
	kindBody = "comment inside function body: rename, extract a helper or name the constant instead"
	kindDoc  = "function doc comment longer than the cap: split the function or trim the doc to its contract"
)

type finding struct {
	pos  token.Position
	kind string
}

var generatedMarker = regexp.MustCompile(`(?m)^// Code generated .* DO NOT EDIT\.$`)

var skippedDirs = map[string]bool{
	".git": true, ".claude": true, ".ignored": true, "node_modules": true,
	"vendor": true, "frontend": true, "testdata": true,
}

func scanTree(root string) (map[string][]finding, error) {
	result := map[string][]finding{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skippedDirs[d.Name()] && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		found, err := scanFile(path)
		if err != nil {
			return err
		}
		if len(found) > 0 {
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return relErr
			}
			result[filepath.ToSlash(rel)] = found
		}
		return nil
	})
	return result, err
}

func scanFile(path string) ([]finding, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	return findings(fset, file), nil
}

func findings(fset *token.FileSet, file *ast.File) []finding {
	if isGenerated(file) {
		return nil
	}
	var found []finding
	bodies := funcBodies(file)
	for _, group := range file.Comments {
		if inAnyBody(bodies, group) && hasProse(group) && !isWhyWithReason(group) && !isSuppression(group) {
			found = append(found, finding{pos: fset.Position(group.Pos()), kind: kindBody})
		}
	}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Doc != nil && proseLines(fn.Doc) > docCap {
			found = append(found, finding{pos: fset.Position(fn.Doc.Pos()), kind: kindDoc})
		}
	}
	return found
}

func isGenerated(file *ast.File) bool {
	for _, group := range file.Comments {
		if group.End() > file.Package {
			break
		}
		if generatedMarker.MatchString(rawText(group)) {
			return true
		}
	}
	return false
}

func rawText(group *ast.CommentGroup) string {
	lines := make([]string, 0, len(group.List))
	for _, c := range group.List {
		lines = append(lines, c.Text)
	}
	return strings.Join(lines, "\n")
}

func funcBodies(file *ast.File) []*ast.BlockStmt {
	var bodies []*ast.BlockStmt
	ast.Inspect(file, func(n ast.Node) bool {
		switch fn := n.(type) {
		case *ast.FuncDecl:
			if fn.Body != nil {
				bodies = append(bodies, fn.Body)
			}
		case *ast.FuncLit:
			bodies = append(bodies, fn.Body)
		}
		return true
	})
	return bodies
}

func inAnyBody(bodies []*ast.BlockStmt, group *ast.CommentGroup) bool {
	for _, body := range bodies {
		if group.Pos() > body.Lbrace && group.End() < body.Rbrace {
			return true
		}
	}
	return false
}

func hasProse(group *ast.CommentGroup) bool {
	for line := range strings.SplitSeq(group.Text(), "\n") {
		if text := strings.TrimSpace(line); text != "" && !isToolDirective(text) {
			return true
		}
	}
	return false
}

var suppressionPrefixes = []string{"coverage-ignore", "nolint", "#nosec", "nosemgrep", "lint:ignore"}

func isToolDirective(text string) bool {
	for _, prefix := range suppressionPrefixes {
		if strings.HasPrefix(text, prefix) {
			return true
		}
	}
	return false
}

// isSuppression reports whether the group opens with a linter or coverage
// directive. Its continuation lines are that directive's justification.
func isSuppression(group *ast.CommentGroup) bool {
	first, _, _ := strings.Cut(strings.TrimSpace(group.Text()), "\n")
	return isToolDirective(first)
}

func isWhyWithReason(group *ast.CommentGroup) bool {
	reason, ok := strings.CutPrefix(strings.TrimSpace(group.Text()), "why:")
	return ok && strings.TrimSpace(reason) != ""
}

func proseLines(group *ast.CommentGroup) int {
	count := 0
	for line := range strings.SplitSeq(group.Text(), "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}
