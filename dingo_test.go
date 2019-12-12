package dingo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type (
	Interface interface {
		Test() int
	}

	InterfaceSub Interface

	InterfaceImpl1 struct {
		i   int
		foo string
	}

	InterfaceImpl2 struct {
		i int
	}

	IfaceProvider          func() Interface
	IfaceWithErrorProvider func() (Interface, error)

	DepTest struct {
		Iface  Interface `inject:""`
		Iface2 Interface `inject:"test"`

		IfaceProvider          IfaceProvider          `inject:""`
		IfaceWithErrorProvider IfaceWithErrorProvider `inject:""`
		IfaceProvided          Interface              `inject:"provider"`
		IfaceImpl1Provided     Interface              `inject:"providerimpl1"`
		IfaceInstance          Interface              `inject:"instance"`
	}

	TestSingleton struct {
		i int
	}

	TestModule struct{}

	PreTestModule struct{}
)

func InterfaceProvider(str string) Interface {
	return &InterfaceImpl1{foo: str}
}

func InterfaceImpl1Provider(str string) *InterfaceImpl1 {
	return &InterfaceImpl1{foo: str}
}

func (ptm *PreTestModule) Configure(injector *Injector) {
	injector.Bind((*string)(nil)).ToInstance("Hello World")
}

func (tm *TestModule) Configure(injector *Injector) {
	injector.Bind((*Interface)(nil)).To((*InterfaceSub)(nil))
	injector.Bind((*InterfaceSub)(nil)).To(InterfaceImpl1{})
	injector.Bind((*Interface)(nil)).AnnotatedWith("test").To(InterfaceImpl2{})

	injector.Bind((*Interface)(nil)).AnnotatedWith("provider").ToProvider(InterfaceProvider)
	injector.Bind((*Interface)(nil)).AnnotatedWith("providerimpl1").ToProvider(InterfaceImpl1Provider)
	injector.Bind((*Interface)(nil)).AnnotatedWith("instance").ToInstance(new(InterfaceImpl2))

	injector.Bind(TestSingleton{}).AsEagerSingleton()
}

func (if1 *InterfaceImpl1) Test() int {
	return 1
}

func (if2 *InterfaceImpl2) Test() int {
	return 2
}

func TestDingoResolving(t *testing.T) {
	t.Run("Should resolve dependencies on request", func(t *testing.T) {
		injector, err := NewInjector(new(PreTestModule), new(TestModule))
		assert.NoError(t, err)

		i, err := injector.GetInstance(new(Interface))
		assert.NoError(t, err)
		var iface Interface
		iface = i.(Interface)

		assert.Equal(t, 1, iface.Test())

		i, err = injector.GetInstance(new(DepTest))
		assert.NoError(t, err)
		dt := *i.(*DepTest)

		assert.Equal(t, 1, dt.Iface.Test())
		assert.Equal(t, 2, dt.Iface2.Test())

		var dt2 DepTest
		assert.NoError(t, injector.requestInjection(&dt2, nil))

		assert.Equal(t, 1, dt2.Iface.Test())
		assert.Equal(t, 2, dt2.Iface2.Test())

		assert.Equal(t, 1, dt.IfaceProvided.Test())
		assert.Equal(t, 1, dt.IfaceImpl1Provided.Test())
		assert.Equal(t, 2, dt.IfaceInstance.Test())

		assert.Equal(t, 1, dt.IfaceProvider().Test())
		iface, err = dt.IfaceWithErrorProvider()
		assert.NoError(t, err)
		assert.Equal(t, 1, iface.Test())
		assert.Equal(t, "Hello World", dt.IfaceProvided.(*InterfaceImpl1).foo)
		assert.Equal(t, "Hello World", dt.IfaceImpl1Provided.(*InterfaceImpl1).foo)
	})

	t.Run("Should resolve scopes", func(t *testing.T) {
		injector, err := NewInjector(new(TestModule))
		assert.NoError(t, err)

		i1, err := injector.GetInstance(TestSingleton{})
		assert.NoError(t, err)
		i2, err := injector.GetInstance(TestSingleton{})
		assert.NoError(t, err)
		assert.Equal(t, i1, i2)
	})

	t.Run("Error cases", func(t *testing.T) {
		var injector *Injector
		_, err := injector.Child()
		assert.Error(t, err)
	})
}

type testBoundNothingProvider func() *InterfaceImpl1

func TestBoundToNothing(t *testing.T) {
	injector, err := NewInjector()
	assert.NoError(t, err)

	injector.Bind(new(InterfaceImpl1)).AnnotatedWith("test")

	i, err := injector.GetInstance(new(testBoundNothingProvider))
	assert.NoError(t, err)
	ii, ok := i.(testBoundNothingProvider)
	assert.True(t, ok)
	assert.NotNil(t, ii)
	assert.NotNil(t, ii())
}

// interceptors
type (
	AopInterface interface {
		Test() string
	}

	AopImpl struct{}

	AopDep struct {
		A AopInterface `inject:""`
	}

	AopInterceptor1 struct {
		AopInterface
	}

	AopInterceptor2 struct {
		AopInterface
	}

	AopModule struct{}
)

func (m *AopModule) Configure(injector *Injector) {
	injector.Bind((*AopInterface)(nil)).To(AopImpl{})

	injector.BindInterceptor((*AopInterface)(nil), AopInterceptor1{})
	injector.BindInterceptor((*AopInterface)(nil), AopInterceptor2{})
}

func (a *AopImpl) Test() string {
	return "Test"
}

func (a *AopInterceptor1) Test() string {
	return a.AopInterface.Test() + " 1"
}

func (a *AopInterceptor2) Test() string {
	return a.AopInterface.Test() + " 2"
}

func TestInterceptors(t *testing.T) {
	injector, err := NewInjector(new(AopModule))
	assert.NoError(t, err)

	var dep AopDep
	assert.NoError(t, injector.requestInjection(&dep, nil))

	assert.Equal(t, "Test 1 2", dep.A.Test())
}

func TestOptional(t *testing.T) {
	type test struct {
		Must      string `inject:"must"`
		Optional  string `inject:"option,optional"`
		Optional2 string `inject:"option, optional"`
	}

	injector, err := NewInjector()
	assert.NoError(t, err)

	_, err = injector.GetInstance(new(test))
	assert.Error(t, err)

	injector.Bind(new(string)).AnnotatedWith("must").ToInstance("must")
	i, err := injector.GetInstance(new(test))
	assert.NoError(t, err)
	assert.Equal(t, i.(*test).Must, "must")
	assert.Equal(t, i.(*test).Optional, "")
	assert.Equal(t, i.(*test).Optional2, "")

	injector.Bind(new(string)).AnnotatedWith("option").ToInstance("option")
	i, err = injector.GetInstance(new(test))
	assert.NoError(t, err)
	assert.Equal(t, i.(*test).Must, "must")
	assert.Equal(t, i.(*test).Optional, "option")
	assert.Equal(t, i.(*test).Optional2, "option")
}
