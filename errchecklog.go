package errchecklog

import (
	"fmt"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"

	"github.com/golangci/plugin-module-register/register"
)

type PluginErrchecklog struct{}

func init() {
	register.Plugin("errchecklog", New)
}

func New(settings any) (register.LinterPlugin, error) {
	return &PluginErrchecklog{}, nil
}

func (p *PluginErrchecklog) BuildAnalyzers() ([]*analysis.Analyzer, error) {
	analyzer := NewAnalyzer()
	return []*analysis.Analyzer{analyzer}, nil
}

func (p *PluginErrchecklog) GetLoadMode() string {
	return register.LoadModeSyntax
}

func NewAnalyzer() *analysis.Analyzer {
	return &analysis.Analyzer{
		Name: "errchecklog",
		Doc:  "reports calls to methods of the Log interface when the concrete implementation comes from a different package",
		Requires: []*analysis.Analyzer{
			buildssa.Analyzer,
		},
		Run: func(pass *analysis.Pass) (interface{}, error) {
			// Locate the Log interface in the current package or in one of the imports.
			logIface, logPkgPath, err := findLogInterface(pass)
			if err != nil {
				return nil, err
			}

			// Build a set of method names declared in the Log interface.
			logMethods := make(map[string]bool)
			for i := 0; i < logIface.NumMethods(); i++ {
				m := logIface.Method(i)
				logMethods[m.Name()] = true
			}

			// Get the SSA representation of the code.
			ssaProg := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)

			// Walk through all functions and instructions in the SSA.
			for _, fn := range ssaProg.SrcFuncs {
				for _, block := range fn.Blocks {
					for _, instr := range block.Instrs {
						call, ok := instr.(*ssa.Call)
						if !ok {
							continue
						}
						// Only check interface calls.
						if !call.Common().IsInvoke() {
							continue
						}

						methodName := call.Common().Method.Name()
						if !logMethods[methodName] {
							continue
						}

						// Resolve the concrete type behind the interface value.
						ifaceVal := call.Common().Value
						concreteType := resolveConcrete(ifaceVal, logIface)
						if concreteType == nil {
							continue
						}

						// Determine the named type to check its package.
						named, _ := derefNamed(concreteType)
						if named == nil {
							continue
						}
						implPkg := named.Obj().Pkg()
						if implPkg == nil {
							continue
						}
						// Report if the concrete type is implemented from a package different than the Log interface.
						if implPkg.Path() != logPkgPath {
							pass.Reportf(instr.Pos(),
								"call to Log method %q on type %v from pkg %q",
								methodName, named.Obj().Name(), implPkg.Path())
						}
					}
				}
			}

			return nil, nil
		},
	}
}

// findLogInterface searches for the "Log" interface in the current package or its imports.
func findLogInterface(pass *analysis.Pass) (*types.Interface, string, error) {
	if iface, ok := lookupInterface(pass.Pkg.Scope(), "Log"); ok {
		return iface, pass.Pkg.Path(), nil
	}
	for _, imp := range pass.Pkg.Imports() {
		if iface, ok := lookupInterface(imp.Scope(), "Log"); ok {
			return iface, imp.Path(), nil
		}
	}
	return nil, "", fmt.Errorf("could not find interface Log")
}

// lookupInterface finds an object with the given name in the provided scope and checks if it's an interface.
func lookupInterface(scope *types.Scope, name string) (*types.Interface, bool) {
	obj := scope.Lookup(name)
	if obj == nil {
		return nil, false
	}
	tn, ok := obj.(*types.TypeName)
	if !ok {
		return nil, false
	}
	iface, ok := tn.Type().Underlying().(*types.Interface)
	return iface, ok
}

// resolveConcrete attempts to determine the concrete type behind an interface value by backtracking SSA instructions.
func resolveConcrete(val ssa.Value, iface *types.Interface) types.Type {
	switch v := val.(type) {
	case *ssa.MakeInterface:
		// The point where a concrete type is wrapped into an interface.
		return v.X.Type()

	case *ssa.UnOp:
		// For operations like &x or *x.
		return resolveConcrete(v.X, iface)

	case *ssa.Field:
		// Accessing a field: x.field.
		return resolveConcrete(v.X, iface)

	case *ssa.FieldAddr:
		// Address of a field: &x.field.
		return resolveConcrete(v.X, iface)

	case *ssa.Convert:
		// Conversion: T(x).
		return resolveConcrete(v.X, iface)

	case *ssa.Call:
		// When a call returns an interface or concrete type.
		if !isInterface(v.Type()) {
			return v.Type()
		}
		for _, arg := range v.Common().Args {
			if types.Implements(arg.Type(), iface) ||
				types.Implements(types.NewPointer(arg.Type()), iface) {
				return arg.Type()
			}
		}
		return v.Type()

	case *ssa.Phi:
		// Phi node: merge of several control flow paths.
		for _, incoming := range v.Edges {
			candidate := resolveConcrete(incoming, iface)
			if candidate != nil {
				return candidate
			}
		}
		return nil

	case *ssa.Extract:
		// Extraction from a multi-value (e.g., (val, err) = someFunc(...)).
		return resolveConcrete(v.Tuple, iface)

	case *ssa.Alloc:
		// Allocation: new(SomeType).
		if allocType, ok := v.Type().(*types.Pointer); ok {
			return allocType.Elem()
		}
		return v.Type()

	default:
		// Fallback to the value's type.
		return v.Type()
	}
}

// isInterface returns true if t's underlying type is an interface.
func isInterface(t types.Type) bool {
	_, ok := t.Underlying().(*types.Interface)
	return ok
}

// derefNamed returns the named type if t is either a named type or a pointer to one.
func derefNamed(t types.Type) (*types.Named, bool) {
	if ptr, ok := t.(*types.Pointer); ok {
		if named, ok2 := ptr.Elem().(*types.Named); ok2 {
			return named, true
		}
	}
	if named, ok := t.(*types.Named); ok {
		return named, true
	}
	return nil, false
}
