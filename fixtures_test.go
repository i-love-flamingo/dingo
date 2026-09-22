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
)
