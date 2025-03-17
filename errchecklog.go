package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// NewAnalyzer constructs an Analyzer parameterized by the package name
// (e.g. "fakefmt") in which the provided interface "Printer" is defined.
func NewAnalyzer(providedPkg string) *analysis.Analyzer {
	return &analysis.Analyzer{
		Name: "callcheck",
		Doc:  "reports calls to methods of the provided interface when implemented by a different package",
		Requires: []*analysis.Analyzer{
			inspect.Analyzer,
		},
		Run: func(pass *analysis.Pass) (interface{}, error) {
			// Precompute a map from variable objects to their initializer expressions.
			initMap := make(map[types.Object]ast.Expr)
			for _, file := range pass.Files {
				ast.Inspect(file, func(n ast.Node) bool {
					switch node := n.(type) {
					case *ast.ValueSpec:
						for i, name := range node.Names {
							if node.Values != nil && i < len(node.Values) {
								if obj := pass.TypesInfo.Defs[name]; obj != nil {
									initMap[obj] = node.Values[i]
								}
							}
						}
					case *ast.AssignStmt:
						// Only consider short variable declarations (:=).
						if node.Tok != token.DEFINE {
							return true
						}
						for i, lhs := range node.Lhs {
							ident, ok := lhs.(*ast.Ident)
							if !ok {
								continue
							}
							if obj := pass.TypesInfo.ObjectOf(ident); obj != nil && i < len(node.Rhs) {
								initMap[obj] = node.Rhs[i]
							}
						}
					}
					return true
				})
			}

			// Lookup the provided interface "Printer" in imported packages.
			var providedIface *types.Interface
			var providedPkgPath string
			for _, imp := range pass.Pkg.Imports() {
				// Match if the import's package name equals providedPkg
				// or its path ends with "/providedPkg".
				if imp.Name() == providedPkg || strings.HasSuffix(imp.Path(), "/"+providedPkg) || imp.Path() == providedPkg {
					obj := imp.Scope().Lookup("Printer")
					if obj == nil {
						continue
					}
					tn, ok := obj.(*types.TypeName)
					if !ok {
						continue
					}
					iface, ok := tn.Type().Underlying().(*types.Interface)
					if !ok {
						continue
					}
					providedIface = iface
					providedPkgPath = imp.Path()
					break
				}
			}
			if providedIface == nil {
				// Fallback: check current package.
				obj := pass.Pkg.Scope().Lookup("Printer")
				if obj != nil {
					if tn, ok := obj.(*types.TypeName); ok {
						if iface, ok2 := tn.Type().Underlying().(*types.Interface); ok2 {
							providedIface = iface
							providedPkgPath = pass.Pkg.Path()
						}
					}
				}
			}
			if providedIface == nil {
				return nil, fmt.Errorf("could not find interface Printer in package %q", providedPkg)
			}

			// Build a set of method names from the provided interface.
			providedMethods := make(map[string]struct{})
			for i := 0; i < providedIface.NumMethods(); i++ {
				m := providedIface.Method(i)
				providedMethods[m.Name()] = struct{}{}
			}

			inspector := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
			nodeFilter := []ast.Node{
				(*ast.CallExpr)(nil),
			}
			inspector.Preorder(nodeFilter, func(n ast.Node) {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return
				}
				// Only interested in methods defined in the provided interface.
				if _, exists := providedMethods[sel.Sel.Name]; !exists {
					return
				}
				// We expect the receiver to be an identifier.
				id, ok := sel.X.(*ast.Ident)
				if !ok {
					return
				}
				obj := pass.TypesInfo.ObjectOf(id)
				if obj == nil {
					return
				}
				// Look up the initializer for this variable.
				initExpr, found := initMap[obj]
				if !found {
					return
				}
				// Determine the concrete type from the initializer.
				concreteType := pass.TypesInfo.TypeOf(initExpr)
				// For a pointer, check the element type.
				ptr, ok := concreteType.(*types.Pointer)
				if !ok {
					return
				}
				named, ok := ptr.Elem().(*types.Named)
				if !ok {
					return
				}
				implPkg := named.Obj().Pkg()
				if implPkg == nil {
					return
				}
				// If the concrete implementation comes from a different package,
				// report a diagnostic.
				if implPkg.Path() != providedPkgPath {
					pass.Reportf(call.Lparen, "call to a provided interface found")
				}
			})
			return nil, nil
		},
	}
}
