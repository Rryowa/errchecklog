// errchecklog/plugin.go
package errchecklog

import (
	"fmt"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"

	"github.com/golangci/plugin-module-register/register"
)

// Config holds the plugin settings.
type Config struct {
	InterfacePackage string `json:"interface_package"`
	InterfaceName    string `json:"interface_name"`
}

// PluginErrchecklog implements register.LinterPlugin.
type PluginErrchecklog struct {
	settings Config
}

// Register the plugin with the module register.
func init() {
	register.Plugin("errchecklog", New)
}

// New decodes the settings and returns an instance of PluginErrchecklog.
func New(settings any) (register.LinterPlugin, error) {
	// Use the generic DecodeSettings function from the register package.
	cfg, err := register.DecodeSettings[Config](settings)
	if err != nil {
		return nil, err
	}
	return &PluginErrchecklog{settings: cfg}, nil
}

// BuildAnalyzers returns the analyzer(s) for the plugin.
func (p *PluginErrchecklog) BuildAnalyzers() ([]*analysis.Analyzer, error) {
	// NewAnalyzer is your function that creates the Analyzer given the interface package and name.
	analyzer := NewAnalyzer(p.settings.InterfacePackage, p.settings.InterfaceName)
	return []*analysis.Analyzer{analyzer}, nil
}

// GetLoadMode returns the load mode for the plugin; adjust if needed.
func (p *PluginErrchecklog) GetLoadMode() string {
	// For example, use LoadModeSyntax to indicate that the linter should work on the syntax tree.
	return register.LoadModeSyntax
}

/*
NewAnalyzer создаёт анализатор (Analyzer), который проверяет вызовы методов интерфейса "Printer"
и сообщает о случаях, когда конкретная реализация этого интерфейса находится в другом пакете
(отличном от пакета, где определён сам интерфейс).

Как это работает в общих чертах:
 1. Мы ищем интерфейс "Printer" в заданном пакете (по названию или пути).
 2. Мы просматриваем SSA-представление кода, чтобы найти все вызовы методов через интерфейс.
 3. Если такой интерфейсный вызов относится к "Printer", но конкретный тип-реализация берётся из другого пакета, мы формируем предупреждение (diagnostic).
*/
func NewAnalyzer(providedPkg, interfaceName string) *analysis.Analyzer {
	return &analysis.Analyzer{
		Name: "errchecklog",
		Doc:  "reports calls to methods of the provided interface when implemented by a different package",
		Requires: []*analysis.Analyzer{
			buildssa.Analyzer,
		},
		Run: func(pass *analysis.Pass) (interface{}, error) {
			// 1) Ищем интерфейс "Printer" в указанном пакете (либо в текущем).
			//    (In code, we're actually using the 'interfaceName' argument now.)
			providedIface, providedPkgPath, err := findPrinterInterface(pass, providedPkg, interfaceName)
			if err != nil {
				return nil, err
			}

			// 2) Собираем названия всех методов, объявленных в этом интерфейсе.
			methodNames := make(map[string]bool)
			for i := 0; i < providedIface.NumMethods(); i++ {
				m := providedIface.Method(i)
				methodNames[m.Name()] = true
			}

			// 3) Получаем SSA-представление кода текущего пакета.
			ssaProg := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)

			// 4) Проходим по всем функциям в SSA-программе и ищем вызовы через интерфейс.
			for _, fn := range ssaProg.SrcFuncs {
				for _, block := range fn.Blocks {
					for _, instr := range block.Instrs {
						call, ok := instr.(*ssa.Call)
						if !ok {
							continue
						}
						// Проверяем, что это вызов именно через интерфейс (IsInvoke).
						if !call.Common().IsInvoke() {
							continue
						}

						// Метод, который вызывается, например "Print" в x.Print(...)
						invokedMethod := call.Common().Method.Name()
						if !methodNames[invokedMethod] {
							continue
						}

						// 5) call.Common().Value — это значение интерфейса;
						//    найдём конкретный тип, который скрывается за интерфейсом.
						ifaceVal := call.Common().Value
						concreteType := resolveConcrete(ifaceVal, providedIface)
						if concreteType == nil {
							// can't resolve => skip
							continue
						}

						// 6) Проверяем, из какого пакета берётся этот конкретный тип;
						named, _ := derefNamed(concreteType)
						if named == nil {
							continue
						}
						implPkg := named.Obj().Pkg()
						if implPkg == nil {
							continue
						}
						if implPkg.Path() != providedPkgPath {
							pass.Reportf(instr.Pos(),
								"call to a provided interface found (method %q on type %v from pkg %q)",
								invokedMethod, named.Obj().Name(), implPkg.Path())
						}
					}
				}
			}

			return nil, nil
		},
	}
}

/*
findPrinterInterface ищет интерфейс с именем "Printer":
  - Сначала в импортированных пакетах, подходящих под providedPkg (по названию или суффиксу пути).
  - Если не находит, то в текущем пакете.

Возвращает:

	(сам Интерфейс, путь пакета Интерфейса, nil)

или ошибку, если Интерфейс не найден.
*/
func findPrinterInterface(pass *analysis.Pass, providedPkg, interfaceName string) (*types.Interface, string, error) {
	// Attempt in imports
	for _, imp := range pass.Pkg.Imports() {
		if imp.Name() == providedPkg ||
			strings.HasSuffix(imp.Path(), "/"+providedPkg) ||
			imp.Path() == providedPkg {
			iface, ok := lookupInterface(imp.Scope(), interfaceName)
			if ok {
				return iface, imp.Path(), nil
			}
		}
	}
	// Attempt in this package
	iface, ok := lookupInterface(pass.Pkg.Scope(), interfaceName)
	if ok {
		return iface, pass.Pkg.Path(), nil
	}
	return nil, "", fmt.Errorf("could not find interface %s in %q", interfaceName, providedPkg)
}

// lookupInterface ищет в заданном scope объект с именем name и проверяет, является ли он интерфейсом.
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

/*
resolveConcrete пытается проследить SSA-инструкции назад(backtrack), чтобы определить реальный
конкретный тип, лежащий за значением интерфейса. Например:
  - *ssa.MakeInterface — точка, где конкретный тип X заворачивается в интерфейс.
  - *ssa.Field / *ssa.FieldAddr — ссылку на поле структуры.
  - *ssa.UnOp / *ssa.Convert / *ssa.Call — когда значение преобразуется, передаётся и т.д.
  - *ssa.Phi — слияние нескольких путей.

Рекурсивно проверяя эти инструкции, мы пытаемся выяснить, какой именно тип реализует интерфейс
*/
func resolveConcrete(val ssa.Value, iface *types.Interface) types.Type {
	switch v := val.(type) {
	case *ssa.MakeInterface:
		// Момент, когда конкретный тип X "оборачивают" в интерфейс.
		return v.X.Type()

	case *ssa.UnOp:
		// Операции типа &x или *x; разбираем, что за x.
		return resolveConcrete(v.X, iface)

	case *ssa.Field:
		// Доступ к полю x.field; смотрим, что за x.
		return resolveConcrete(v.X, iface)

	case *ssa.FieldAddr:
		// Адрес поля x.field; тоже смотрим, что за x.
		return resolveConcrete(v.X, iface)

	case *ssa.Convert:
		// Преобразование T(x); идём к x.
		return resolveConcrete(v.X, iface)

	case *ssa.Call:
		// Вызов может вернуть структуру, интерфейс и т.д
		// Если возвращённый тип не является интерфейсом, значит это уже конкретика
		if !isInterface(v.Type()) {
			return v.Type()
		}
		// Если всё ещё интерфейс, иногда в аргументах вызова скрывается конкретный тип
		for _, arg := range v.Common().Args {
			if types.Implements(arg.Type(), iface) ||
				types.Implements(types.NewPointer(arg.Type()), iface) {
				return arg.Type()
			}
		}
		return v.Type()

	case *ssa.Phi:
		// Слияние нескольких значений в разных флоу
		// Пробуем каждый Edge
		for _, incoming := range v.Edges {
			candidate := resolveConcrete(incoming, iface)
			if candidate != nil {
				return candidate
			}
		}
		return nil

	case *ssa.Extract:
		// Извлечение из многозначного результата (val, err) = someFunc(...)
		return resolveConcrete(v.Tuple, iface)

	case *ssa.Alloc:
		// new(SomeType) => указатель на SomeType
		if allocType, ok := v.Type().(*types.Pointer); ok {
			return allocType.Elem()
		}
		return v.Type()

	default:
		// Если паттерны выше не подошли, возвращаем val.Type().
		// Он может оставаться интерфейсом, если глубже проследить не удалось.
	}
	return val.Type()
}

// isInterface returns true if t.Underlying() is an interface.
func isInterface(t types.Type) bool {
	_, ok := t.Underlying().(*types.Interface)
	return ok
}

/*
derefNamed возвращает *types.Named, если t — это либо именованный тип, либо указатель на
именованный тип (например, *MyStruct). Это полезно, чтобы узнать пакет (Pkg) типа.
*/
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
