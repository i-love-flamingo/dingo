package contract_test

import (
	"testing"

	"flamingo.me/dingo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// GetInstance takes a pointer to the type, a pointer to a slice for a
// multibinding, or a value of a struct type, whichever the caller has at hand.
func TestGetInstance_AcceptsTheFormsConsumersUse(t *testing.T) {
	t.Parallel()

	injector := newInjector(t, dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.Bind(new(greeter)).To(helloGreeter{})
		injector.Bind(new(greeter)).AnnotatedWith("loud").To(loudGreeter{})
		injector.BindMulti(new(greeter)).To(helloGreeter{})
		injector.BindMulti(new(greeter)).To(loudGreeter{})
	}))

	multi, err := injector.GetInstance(new([]greeter))
	require.NoError(t, err)

	greetings := make([]string, 0, 2)

	for _, item := range multi.([]greeter) {
		greetings = append(greetings, item.Greet())
	}

	assert.Equal(t, []string{"hello", "HELLO"}, greetings)

	service, err := injector.GetInstance(providedService{})
	require.NoError(t, err)
	assert.Equal(t, "hello", service.(*providedService).Greeter.Greet())

	annotated, err := injector.GetAnnotatedInstance(new(greeter), "loud")
	require.NoError(t, err)
	assert.Equal(t, "HELLO", annotated.(greeter).Greet())
}
