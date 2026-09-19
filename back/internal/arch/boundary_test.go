// Package arch_test keeps the dependency direction between the layers honest.
package arch_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const modulePath = "github.com/moxicom/cursed_matrix/back"

func TestDependencyDirection(t *testing.T) {
	tests := []struct {
		name  string
		root  string
		allow func(path string) bool
		why   string
	}{
		{
			name: "domain imports nothing but the standard library",
			root: "../domain",
			allow: func(path string) bool {
				if !strings.Contains(path, ".") { // no dot in the first segment: stdlib
					return true
				}
				if strings.HasPrefix(path, modulePath+"/internal/domain/") {
					return true
				}
				return path == "github.com/google/uuid"
			},
			why: "the domain must stay independent of transport, storage and frameworks",
		},
		{
			name: "application depends on the domain, never on an adapter",
			root: "../app",
			allow: func(path string) bool {
				if !strings.Contains(path, ".") {
					return true
				}
				if strings.HasPrefix(path, modulePath+"/internal/domain/") ||
					strings.HasPrefix(path, modulePath+"/internal/app/") {
					return true
				}
				return path == "github.com/google/uuid"
			},
			why: "a service that imports an adapter cannot be run against a different one",
		},
		{
			name: "the logger package depends on nothing internal",
			root: "../../pkg",
			allow: func(path string) bool {
				return !strings.HasPrefix(path, modulePath+"/internal/")
			},
			why: "pkg/ is meant to be reusable on its own",
		},
		{
			name: "adapters do not import each other",
			root: "../adapter",
			allow: func(path string) bool {
				return !strings.HasPrefix(path, modulePath+"/internal/adapter/")
			},
			why: "PostgreSQL and Redis must be replaceable one at a time",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			violations := imports(t, tc.root, tc.allow)
			for _, v := range violations {
				t.Errorf("%s imports %s\n  %s", v.file, v.path, tc.why)
			}
		})
	}
}

type violation struct {
	file string
	path string
}

func imports(t *testing.T, root string, allow func(string) bool) []violation {
	t.Helper()

	var found []violation
	err := filepath.Walk(root, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}

		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}

		for _, imported := range file.Imports {
			unquoted, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				return err
			}
			if !allow(unquoted) {
				found = append(found, violation{file: path, path: unquoted})
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return found
}

func TestPortsDeclareInterfacesOnly(t *testing.T) {
	const dir = "../app/port"

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}

		t.Run(entry.Name(), func(t *testing.T) {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, filepath.Join(dir, entry.Name()), nil, 0)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}

			for _, decl := range file.Decls {
				switch typed := decl.(type) {
				case *ast.FuncDecl:
					t.Errorf("%s declares a function; a port is an interface, and behaviour belongs "+
						"in the domain or in an adapter", typed.Name.Name)
				case *ast.GenDecl:
					if typed.Tok != token.TYPE && typed.Tok != token.IMPORT {
						t.Errorf("%s declares a %s; a port declares only interfaces", entry.Name(), typed.Tok)
						continue
					}
					for _, spec := range typed.Specs {
						typeSpec, ok := spec.(*ast.TypeSpec)
						if !ok {
							continue
						}
						if _, ok := typeSpec.Type.(*ast.InterfaceType); !ok {
							t.Errorf("type %s is not an interface; its vocabulary belongs in the domain "+
								"or in a package of its own", typeSpec.Name.Name)
						}
					}
				}
			}
		})
	}
}
