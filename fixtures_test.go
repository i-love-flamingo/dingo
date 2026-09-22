package dingo_test

// Fixtures shared by the black-box tests in this package.
//
// Types used in identity assertions carry at least one field on purpose:
// reflect.New of a zero-sized type returns runtime.zerobase, so two
// independent allocations of an empty struct are the same pointer and an
// assertion on them holds with or without a scope.

type (
	// greeter is the interface most tests bind.
	greeter interface {
		Greet() string
	}

	// counter is the identity fixture: one field, so two allocations differ.
	counter struct {
		N int
	}

	// helloGreeter is the default greeter implementation.
	helloGreeter struct {
		Name string
	}

	// loudGreeter is a second implementation, for precedence and
	// disjointness assertions.
	loudGreeter struct {
		Volume int
	}

	// greeterSlice injects a multibinding.
	greeterSlice struct {
		All []greeter `inject:""`
	}

	// annotatedGreeterSlice injects an annotated and an unannotated
	// multibinding of the same type side by side.
	annotatedGreeterSlice struct {
		Plain     []greeter `inject:""`
		Annotated []greeter `inject:"loud"`
	}

	// greeterMap injects a map binding.
	greeterMap struct {
		All map[string]greeter `inject:""`
	}

	// annotatedGreeterMap injects an annotated and an unannotated map
	// binding of the same type side by side.
	annotatedGreeterMap struct {
		Plain     map[string]greeter `inject:""`
		Annotated map[string]greeter `inject:"loud"`
	}
)

func (g *helloGreeter) Greet() string {
	return "hello"
}

func (g loudGreeter) Greet() string {
	return "HELLO"
}

func keysOf(m map[string]greeter) []string {
	keys := make([]string, 0, len(m))

	for key := range m {
		keys = append(keys, key)
	}

	return keys
}
