<p align="center"><img src="https://raw.githubusercontent.com/go-vet-analyzers/brand/main/social/go-vet-analyzers.png" alt="go-vet-analyzers/respondto" width="720"></p>

# go-vet-analyzers/respondto

[![ci](https://github.com/go-vet-analyzers/respondto/actions/workflows/ci.yml/badge.svg)](https://github.com/go-vet-analyzers/respondto/actions/workflows/ci.yml)

A `go vet` analyzer that makes the [go-composites](https://github.com/go-composites)
composition style's **reflective method-name strings compile-checked**.

In the composition style, an object is asked whether it implements a method *by
name*, using a string and reflection:

```go
Object.RespondTo(instance, "Message") // package-level helper
instance.RespondTo("Message")         // method form
```

Because the name is a plain string, a typo or a stale name compiles fine and
only blows up at runtime. `respondto` resolves the receiver/object's type and
flags a string-literal method name that is **not in that type's method set** —
turning "the string must match a real method" from human discipline into a check
that **fails CI**.

## Why

`RespondTo`, `Methods()`, and `Send`-style dispatch are what give the composition
style its dynamic, message-passing feel. The cost is that the method name lives
in a string the compiler never looks at: rename a method, miss one call site, and
the program keeps building. `respondto` closes that gap statically.

## Install & use

```sh
go install github.com/go-vet-analyzers/respondto/cmd/respondto@latest
go vet -vettool=$(which respondto) ./...
```

It reports, for example:

```
src/error.go:51:23: RespondTo: Interface has no method "Mesage" (stale or misspelled method name)
```

## What it flags

A call whose callee is named `RespondTo`, in either of the two go-composites
shapes, when the method-name argument is a **string literal** naming a method
that the resolved type does **not** have (looked up across the full method set,
including embedded/promoted methods):

- **package-level** — `RespondTo(object, "Method")`: the type of the first
  argument is resolved.
- **method form** — `x.RespondTo("Method")`: the type of the receiver `x` is
  resolved.

## Limits (deliberately conservative — false positives ~ 0)

It reports **only** when it can be certain, and stays silent otherwise:

- the callee must be named exactly `RespondTo`;
- the method-name argument must be a **string constant** — a variable, a
  field, or any computed expression is ignored;
- the object/receiver type must resolve to a **named type or a non-empty
  interface** with a determinable method set; an empty interface
  (`interface{}`/`any`), a type parameter, or an unnamed non-interface type is
  ignored — those genuinely expose no static method set to judge against;
- promoted methods from embedded interfaces/structs count as present.

Everything outside that envelope (dynamic names, unresolved types) is left alone,
so the analyzer never guesses.

## CI

Add to each repo's workflow:

```yaml
  - run: go install github.com/go-vet-analyzers/respondto/cmd/respondto@latest
  - run: go vet -vettool="$(go env GOPATH)/bin/respondto" ./...
```

## License

BSD-3-Clause © the go-vet-analyzers/respondto authors.
