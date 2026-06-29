// Command respondto runs the respondto analyzer standalone or as a `go vet` tool:
//
//	go install github.com/go-vet-analyzers/respondto/cmd/respondto@latest
//	go vet -vettool=$(which respondto) ./...
package main

import (
	"github.com/go-vet-analyzers/respondto"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() { singlechecker.Main(respondto.Analyzer) }
