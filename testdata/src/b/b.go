// Package b stands in for go-composites' Object/Base package, so package a can
// exercise the qualified call form b.RespondTo(object, "Method").
package b

import "reflect"

func RespondTo(object interface{}, methodNames ...string) bool {
	for _, m := range methodNames {
		if !reflect.ValueOf(object).MethodByName(m).IsValid() {
			return false
		}
	}
	return true
}
