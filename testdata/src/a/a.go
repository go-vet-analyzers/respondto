package a

import (
	"reflect"

	"b"
)

// --- a local stand-in for go-composites' Object/Base package ---------------
// RespondTo(object, name) is the package-level reflective helper the
// composition style uses; the analyzer resolves the first argument's type. It
// is declared variadic so the fixtures can also exercise the analyzer's
// defensive "too few arguments" guards with calls that still type-check.

func RespondTo(object interface{}, methodNames ...string) bool {
	for _, m := range methodNames {
		if !reflect.ValueOf(object).MethodByName(m).IsValid() {
			return false
		}
	}
	return true
}

// --- types under test -------------------------------------------------------

// Greeter is a named interface with a known method set.
type Greeter interface {
	Hello() string
	Bye() string
}

// Base contributes Ping via embedding.
type Base interface{ Ping() string }

// Derived gets Ping through embedding Base, plus its own Bar.
type Derived interface {
	Base
	Bar() int
}

type widget struct{}

func (widget) Hello() string { return "hi" }
func (widget) Bye() string   { return "bye" }
func (widget) RespondTo(methodName string) bool {
	return RespondTo(widget{}, methodName)
}

// gadget carries a pointer method to exercise pointer-receiver resolution.
type gadget struct{}

func (*gadget) Run() string { return "go" }

// --- exercises --------------------------------------------------------------

func packageLevel(g Greeter, d Derived) {
	_ = RespondTo(g, "Hello") // ok: Greeter has Hello
	_ = RespondTo(g, "Nope")  // want `RespondTo: Greeter has no method "Nope"`
	_ = RespondTo(d, "Ping")  // ok: Ping is promoted from embedded Base
	_ = RespondTo(d, "Gone")  // want `RespondTo: Derived has no method "Gone"`
}

func methodForm() {
	w := widget{}
	_ = w.RespondTo("Hello") // ok: widget has Hello
	_ = w.RespondTo("Typo")  // want `RespondTo: widget has no method "Typo"`
}

func qualified(g Greeter) {
	_ = b.RespondTo(g, "Bye")  // ok: qualified package-level call, Bye exists
	_ = b.RespondTo(g, "Nada") // want `RespondTo: Greeter has no method "Nada"`
	_ = b.RespondTo(g)         // ok: qualified call with < 2 args, ignored
}

func pointers(g *gadget, w widget) {
	_ = RespondTo(g, "Run")   // ok: pointer to named struct, Run is a ptr method
	_ = RespondTo(g, "Walk")  // want `RespondTo: gadget has no method "Walk"`
	_ = RespondTo(w, "Hello") // ok: named struct value, Hello exists
	_ = RespondTo(w, "Halt")  // want `RespondTo: widget has no method "Halt"`
}

func dynamic(g Greeter, name string) {
	_ = RespondTo(g, name)         // ok: non-literal argument, ignored
	_ = RespondTo(g, "He"+"llo")   // ok: folds to a real method name
	var any interface{} = g
	_ = RespondTo(any, "Whatever") // ok: empty interface, no method set to judge
	var p *int
	_ = RespondTo(p, "Nope")       // ok: pointer to unnamed type, no method set
}

// --- branches that must NOT be treated as a RespondTo call ------------------

// notRespondTo is an unqualified identifier call whose name is not RespondTo;
// the analyzer ignores it (Ident path, name mismatch).
func notRespondTo(g Greeter, _ string) bool { return g.Hello() != "" }

// shadowsRespondTo exercises the path where an unqualified identifier IS named
// RespondTo but does not resolve to a package-level function: here it is a
// local function value (closure), so isPackageLevel is false and it is ignored.
func shadowsRespondTo(g Greeter) {
	RespondTo := func(object interface{}, methodName string) bool {
		_ = object
		_ = methodName
		return false
	}
	_ = RespondTo(g, "Nope") // ok: local closure, not the package-level helper
}

func notCalls(g Greeter) {
	_ = notRespondTo(g, "Nope")       // ok: identifier call, not named RespondTo
	_ = RespondTo(g)                  // ok: package-level RespondTo with < 2 args
	makeRespondTo()(g, "Nope")        // ok: call-of-a-call, Fun is not Ident/Sel
	g.Hello()                         // ok: selector call, Sel name != RespondTo
}

// makeRespondTo returns a function value so makeRespondTo()(...) yields a call
// whose Fun is itself a *ast.CallExpr (the default switch branch).
func makeRespondTo() func(interface{}, ...string) bool { return RespondTo }

// holder has a field named RespondTo (not a method), exercising the selector
// path where Sel resolves to something that is not a *types.Func
// (selectedPackageFunc returns ok=false). The selector then falls through to
// the method-form interpretation with holder as the receiver, which has no
// such method, so the call is still reported against holder.
type holder struct {
	RespondTo func(string) bool
}

func fieldSelector(h holder) {
	_ = h.RespondTo("Nope") // want `RespondTo: holder has no method "Nope"`
}

// variadicResponder has a variadic RespondTo method so it can be called with
// zero arguments, exercising the method-form path with < 1 arg.
type variadicResponder struct{}

func (variadicResponder) RespondTo(names ...string) bool { return len(names) == 0 }

func methodNoArgs() {
	v := variadicResponder{}
	_ = v.RespondTo() // ok: method form with < 1 arg, ignored
}

// --- unnamed non-empty interface (named-type resolution falls through) ------

func unnamedInterface(x interface {
	Shout() string
}) {
	_ = RespondTo(x, "Shout")   // ok: unnamed interface, Shout exists
	_ = RespondTo(x, "Whisper") // want `RespondTo: interface{Shout\(\) string} has no method "Whisper"`
}
