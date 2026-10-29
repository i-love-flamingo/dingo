package dingo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type (
	childIface       interface{}
	childParentIface interface {
		child() childIface
	}

	childIfaceProvider func() childIface

	childParentIfaceImpl struct {
		childInstance childIfaceProvider
	}
	childIfaceImpl struct{}
)

func (i *childParentIfaceImpl) Inject(childInstance childIfaceProvider) {
	i.childInstance = childInstance
}

func (i *childParentIfaceImpl) child() childIface {
	return i.childInstance()
}

func TestChild(t *testing.T) {
	injector, err := NewInjector()
	assert.NoError(t, err)
	injector.Bind(new(childParentIface)).To(new(childParentIfaceImpl))

	child, err := injector.Child()
	assert.NoError(t, err)
	child.Bind(new(childIface)).To(new(childIfaceImpl))

	_, err = injector.GetInstance(new(childParentIface))
	assert.NoError(t, err)

	// we can get an instance in child, because we have a binding here
	i, err := child.GetInstance(new(childParentIface))
	assert.NoError(t, err)

	assert.NotNil(t, i.(childParentIface).child())
}

func TestChildMapBindingFromParent(t *testing.T) {
	t.Parallel()

	injector, err := NewInjector()
	assert.NoError(t, err)
	injector.BindMap(new(string), "key").ToInstance("parent")
	assert.NoError(t, injector.InitModules())

	child, err := injector.Child()
	assert.NoError(t, err)
	assert.NoError(t, child.InitModules())

	i, err := child.GetAnnotatedInstance(new(string), "map:key")
	assert.NoError(t, err)
	assert.Equal(t, "parent", i)

	var target struct {
		Value string `inject:"map:key"`
	}
	assert.NoError(t, child.RequestInjection(&target))
	assert.Equal(t, "parent", target.Value)
}
