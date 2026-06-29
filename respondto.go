// Package respondto provides a go/analysis analyzer that makes the
// go-composites reflective method-name strings COMPILE-CHECKED.
//
// In the composition style, code asks an object whether it implements a method
// by name, using a string literal and reflection:
//
//	Object.RespondTo(instance, "Message") // package-level helper
//	instance.RespondTo("Message")         // method form
//
// (see github.com/go-composites/error/src/object and .../base/src). Because the
// method name is a plain string, a typo or a stale name compiles fine and only
// fails at runtime — exactly the kind of fragility a static check should remove.
//
// respondto flags a RespondTo call whose method-name argument is a string
// literal naming a method that does NOT exist on the resolved receiver/object
// type's method set. It turns "the string must match a real method" from a
// matter of human discipline into a check that fails CI.
//
// # Scope and conservatism (false positives ~ 0)
//
// The analyzer only reports when ALL of the following hold, so it never guesses:
//
//   - the callee is named exactly RespondTo (either a package-level function
//     RespondTo(object, name) or a method x.RespondTo(name));
//   - the method-name argument is a string literal (a constant string); a
//     variable, computed expression, or non-literal is ignored;
//   - the receiver/object type resolves to a named type or interface that has a
//     determinable method set; an unnamed type, a type parameter, or anything it
//     cannot resolve is ignored;
//   - that type has NO method of the given name, looked up across the full method
//     set including embedded/promoted methods (via types.LookupFieldOrMethod).
//
// Anything outside that envelope (dynamic names, unresolved types, empty
// interfaces such as interface{}/any which expose no methods) is deliberately
// left alone.
package respondto

import (
	"go/ast"
	"go/constant"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

// Doc is the analyzer documentation shown by `go vet`/unitchecker.
const Doc = `report RespondTo("Method") string literals naming a method that does not exist

The go-composites composition style asks objects whether they implement a method
by name, via reflection: Object.RespondTo(x, "Method") or x.RespondTo("Method").
Because the name is a string, a typo or a stale name compiles but fails at
runtime. This analyzer resolves the receiver/object's type and, when the name is
a string literal and the type has a determinable method set, reports a name that
is not in that method set. It is conservative: dynamic names and unresolvable
types are left alone, so false positives are minimal.`

// Analyzer is the respondto analyzer.
var Analyzer = &analysis.Analyzer{
	Name: "respondto",
	Doc:  Doc,
	Run:  run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	for _, f := range pass.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			checkCall(pass, call)
			return true
		})
	}
	return nil, nil
}

// checkCall inspects a single call expression for the RespondTo-style pattern.
func checkCall(pass *analysis.Pass, call *ast.CallExpr) {
	// Three syntactic shapes resolve to a "RespondTo" callee:
	//
	//   pkg.RespondTo(object, "Method")  -> qualified package-level function;
	//                                       the object is the first argument.
	//   RespondTo(object, "Method")      -> unqualified package-level function
	//                                       (call from the same package);
	//                                       the object is the first argument.
	//   x.RespondTo("Method")            -> method; the object is the selector
	//                                       base expression x.
	var objType types.Type
	var nameArg ast.Expr

	switch fun := call.Fun.(type) {
	case *ast.Ident:
		// Unqualified RespondTo(object, name): must resolve to a package-level
		// function literally named RespondTo.
		if fun.Name != "RespondTo" {
			return
		}
		obj, isFunc := pass.TypesInfo.Uses[fun].(*types.Func)
		if !isFunc || !isPackageLevel(obj) {
			return
		}
		if len(call.Args) < 2 {
			return
		}
		objType = pass.TypesInfo.TypeOf(call.Args[0])
		nameArg = call.Args[1]

	case *ast.SelectorExpr:
		if fun.Sel.Name != "RespondTo" {
			return
		}
		if _, isFunc := selectedPackageFunc(pass, fun); isFunc {
			// Qualified package-level RespondTo(object, name).
			//
			// A package-qualified selector (pkg.RespondTo) that resolves to a
			// *types.Func with no receiver is necessarily a package-level
			// function — selectedPackageFunc already guarantees the no-receiver
			// part — so no additional isPackageLevel check is needed here. (The
			// unqualified Ident path below still needs that check, because an
			// unqualified RespondTo could be a local closure.)
			if len(call.Args) < 2 {
				return
			}
			objType = pass.TypesInfo.TypeOf(call.Args[0])
			nameArg = call.Args[1]
		} else {
			// Method form x.RespondTo("Method"): receiver is the selector base.
			if len(call.Args) < 1 {
				return
			}
			objType = pass.TypesInfo.TypeOf(fun.X)
			nameArg = call.Args[0]
		}

	default:
		return
	}

	name, ok := stringLiteralValue(pass, nameArg)
	if !ok {
		return // non-literal argument: ignored (conservative)
	}

	named, ok := resolvableMethodSet(objType)
	if !ok {
		return // type with no determinable method set: ignored
	}

	if obj, _, _ := types.LookupFieldOrMethod(named, true, methodPkg(named), name); obj != nil {
		if _, isFn := obj.(*types.Func); isFn {
			return // method exists — fine
		}
	}

	pass.Reportf(nameArg.Pos(),
		"RespondTo: %s has no method %q (stale or misspelled method name)",
		typeName(named), name)
}

// selectedPackageFunc returns the *types.Func a selector resolves to, when the
// selector is a use of a package-level function (e.g. Object.RespondTo). It
// returns ok=false for method selectors, which are handled via the receiver.
func selectedPackageFunc(pass *analysis.Pass, sel *ast.SelectorExpr) (*types.Func, bool) {
	use := pass.TypesInfo.Uses[sel.Sel]
	fn, ok := use.(*types.Func)
	if !ok {
		return nil, false
	}
	// A package-level function has no receiver; a method does.
	if sig, ok := fn.Type().(*types.Signature); ok && sig.Recv() != nil {
		return nil, false
	}
	return fn, true
}

// isPackageLevel reports whether fn is a package-scoped function (it is the
// object its own package scope binds to its name), as opposed to a local
// closure assigned to a name. A func with no package (a predeclared/universe
// func) is never package-level; the nil-package case is handled by the short
// circuit, since a nil *types.Package compares unequal to fn.
func isPackageLevel(fn *types.Func) bool {
	pkg := fn.Pkg()
	return pkg != nil && pkg.Scope().Lookup(fn.Name()) == fn
}

// stringLiteralValue returns the constant string value of e when e is a string
// constant (a literal, or a const that folds to a string). ok is false
// otherwise — that is the signal to skip dynamic names.
func stringLiteralValue(pass *analysis.Pass, e ast.Expr) (string, bool) {
	tv, ok := pass.TypesInfo.Types[e]
	if !ok || tv.Value == nil || tv.Value.Kind() != constant.String {
		return "", false
	}
	return constant.StringVal(tv.Value), true
}

// resolvableMethodSet returns a type whose method set we trust to be complete
// enough to assert a name is absent. We require a named type or a (named or
// literal) non-empty interface; an empty interface, a type parameter, or an
// unnamed non-interface type yields ok=false (we will not report on those).
func resolvableMethodSet(t types.Type) (types.Type, bool) {
	// Unwrap a single pointer: *T and T share the same value-method names plus
	// pointer methods; LookupFieldOrMethod handles both, so keep the pointer.
	switch u := t.Underlying().(type) {
	case *types.Interface:
		if u.Empty() {
			return nil, false // interface{}/any: exposes no methods, can't judge
		}
		return t, true
	}

	// Non-interface: only trust named types (struct/defined) and their pointers.
	base := t
	if p, ok := t.(*types.Pointer); ok {
		base = p.Elem()
	}
	if _, ok := base.(*types.Named); ok {
		return t, true
	}
	return nil, false
}

// methodPkg returns the package used for unexported-method visibility during
// lookup. Method names in RespondTo are conventionally exported, but passing the
// declaring package keeps lookup correct for unexported ones too.
func methodPkg(t types.Type) *types.Package {
	base := t
	if p, ok := t.(*types.Pointer); ok {
		base = p.Elem()
	}
	if n, ok := base.(*types.Named); ok {
		return n.Obj().Pkg()
	}
	return nil
}

// typeName renders a readable type name for diagnostics.
func typeName(t types.Type) string {
	base := t
	if p, ok := t.(*types.Pointer); ok {
		base = p.Elem()
	}
	if n, ok := base.(*types.Named); ok {
		return n.Obj().Name()
	}
	return t.String()
}
