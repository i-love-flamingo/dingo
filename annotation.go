package dingo

import (
	"reflect"
	"strings"
)

var errorType = reflect.TypeOf(new(error)).Elem()

// parseInjectTag splits an `inject:"..."` tag into the annotation name and
// its options. The first comma-separated part is the name; "optional" among
// the remaining options marks the dependency as optional.
func parseInjectTag(tag string) (name string, optional bool) {
	for _, option := range strings.Split(tag, ",") {
		if strings.TrimSpace(option) == "optional" {
			optional = true
		}
	}

	return strings.Split(tag, ",")[0], optional
}

// mapAnnotationKey returns the mapbinding key of a "map:key" annotation.
func mapAnnotationKey(annotation string) (string, bool) {
	if len(annotation) > 4 && annotation[:4] == "map:" {
		return annotation[4:], true
	}

	return "", false
}

// isProviderType reports whether the type is a provider request such as
// `func() MyInstance`: a function whose declared type name ends in Provider.
func isProviderType(typ reflect.Type) bool {
	return typ.Kind() == reflect.Func && strings.HasSuffix(typ.Name(), "Provider")
}

// providerCanError reports whether a provider type declares a second,
// error-typed result. Only meaningful for types where isProviderType holds.
func providerCanError(typ reflect.Type) bool {
	return typ.NumOut() == 2 && typ.Out(1).AssignableTo(errorType)
}
