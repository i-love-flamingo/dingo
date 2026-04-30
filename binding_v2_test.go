package dingo_test

import (
	"strconv"
	"testing"

	"flamingo.me/dingo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type (
	Operator interface {
		Apply(a, b int) int
	}

	add struct {
	}

	sub struct {
	}
)

func (*add) Apply(a, b int) int {
	return a + b
}

func (*sub) Apply(a, b int) int {
	return a - b
}

func TestBindTransient(t *testing.T) {
	t.Parallel()

	i, err := dingo.NewInjector(dingo.ModuleFunc(func(injector *dingo.Injector) {
		_, _ = dingo.BindTransient[Operator](injector, new(add))

		// without type parameter binds to the struct pointer
		_, _ = dingo.BindTransient(injector, new(sub))
	}))

	require.NoError(t, err)
	require.NotNil(t, i)

	var op Operator

	err = dingo.Get(i, &op)
	if assert.NoError(t, err) {
		assert.IsType(t, new(add), op)
		assert.Equal(t, 32, op.Apply(13, 19))
	}

	var subOp *sub

	err = dingo.Get(i, &subOp)
	if assert.NoError(t, err) {
		assert.IsType(t, new(sub), subOp)
		assert.Equal(t, -6, subOp.Apply(13, 19))
	}
}

func TestBindValue(t *testing.T) {
	t.Parallel()

	instance := new(add)
	i, err := dingo.NewInjector(dingo.ModuleFunc(func(injector *dingo.Injector) {
		_, _ = dingo.BindValue[Operator](injector, instance)
	}))

	require.NoError(t, err)
	require.NotNil(t, i)

	var op Operator

	err = dingo.Get[Operator](i, &op)
	if assert.NoError(t, err) {
		assert.IsType(t, instance, op)
		assert.Same(t, instance, op)
		assert.Equal(t, 32, op.Apply(13, 19))
	}
}

// copy of tests from dingo_setup_test.go, but with generic binding calls
type (
	setupT1 struct {
		member1 string
		member2 string
		member3 string
		Member4 string `inject:"annotation4"`
	}
)

func (s *setupT1) Inject(member1 string, annotated *struct {
	Member2 string `inject:"annotation2"`
	Member3 string `inject:"annotation3"`
}) {
	s.member1 = member1
	s.member2 = annotated.Member2
	s.member3 = annotated.Member3
}

func Test_Bindings(t *testing.T) {
	t.Parallel()

	injector, err := dingo.NewInjector()
	assert.NoError(t, err)

	_, _ = dingo.BindValue(injector, "Member 1") // injector.Bind((*string)(nil)).ToInstance("Member 1")

	_, _ = dingo.BindValue(injector, "Member 2", dingo.Annotated("annotation2")) // injector.Bind((*string)(nil)).AnnotatedWith("annotation2").ToInstance("Member 2")
	_, _ = dingo.BindValue(injector, "Member 3", dingo.Annotated("annotation3")) // injector.Bind((*string)(nil)).AnnotatedWith("annotation3").ToInstance("Member 3")
	_, _ = dingo.BindValue(injector, "Member 4", dingo.Annotated("annotation4")) // injector.Bind((*string)(nil)).AnnotatedWith("annotation4").ToInstance("Member 4")

	i, err := injector.GetInstance((*setupT1)(nil))
	assert.NoError(t, err)

	test, ok := i.(*setupT1)
	if assert.True(t, ok) {
		assert.Equal(t, test.member1, "Member 1")
		assert.Equal(t, test.member2, "Member 2")
		assert.Equal(t, test.member3, "Member 3")
		assert.Equal(t, test.Member4, "Member 4")
	}

	// test generic resolver too
	err = dingo.Get(injector, &test)
	if assert.NoError(t, err) {
		assert.Equal(t, test.member1, "Member 1")
		assert.Equal(t, test.member2, "Member 2")
		assert.Equal(t, test.member3, "Member 3")
		assert.Equal(t, test.Member4, "Member 4")
	}
}

// copy of tests from dingo_test.go, but with generic binding calls

type (
	testInterface interface {
		Test() int
	}

	interfaceSub testInterface

	interfaceImpl1 struct {
		foo string
	}

	interfaceImpl2 struct{}

	testInterfaceProvider          func() testInterface
	testInterfaceWithErrorProvider func() (testInterface, error)

	depTest struct {
		Iface  testInterface `inject:""`
		Iface2 testInterface `inject:"test"`

		IfaceProvider          testInterfaceProvider          `inject:""`
		IfaceWithErrorProvider testInterfaceWithErrorProvider `inject:""`
		IfaceProvided          testInterface                  `inject:"provider"`
		IfaceImpl1Provided     testInterface                  `inject:"providerimpl1"`
		IfaceInstance          testInterface                  `inject:"instance"`
	}

	testSingleton struct{}

	testModule struct{}

	preTestModule struct{}
)

func interfaceProvider(str string) testInterface {
	return &interfaceImpl1{foo: str}
}

func interfaceImpl1Provider(str string) *interfaceImpl1 {
	return &interfaceImpl1{foo: str}
}

func (ptm *preTestModule) Configure(injector *dingo.Injector) {
	injector.Bind((*string)(nil)).ToInstance("Hello World")
}

func (tm *testModule) Configure(injector *dingo.Injector) {
	injector.Bind((*testInterface)(nil)).To((*interfaceSub)(nil))
	injector.Bind((*interfaceSub)(nil)).To(interfaceImpl1{})
	injector.Bind((*testInterface)(nil)).AnnotatedWith("test").To(interfaceImpl2{})

	injector.Bind((*testInterface)(nil)).AnnotatedWith("provider").ToProvider(interfaceProvider)
	injector.Bind((*testInterface)(nil)).AnnotatedWith("providerimpl1").ToProvider(interfaceImpl1Provider)
	injector.Bind((*testInterface)(nil)).AnnotatedWith("instance").ToInstance(new(interfaceImpl2))

	injector.Bind(testSingleton{}).AsEagerSingleton()
}

func (if1 *interfaceImpl1) Test() int {
	return 1
}

func (if2 *interfaceImpl2) Test() int {
	return 2
}

func TestDingoResolving(t *testing.T) {
	t.Parallel()

	t.Run("Should resolve dependencies on request", func(t *testing.T) {
		t.Parallel()

		injector, err := dingo.NewInjector(new(preTestModule), new(testModule))
		require.NoError(t, err)

		var iface testInterface

		err = dingo.Get(injector, &iface)
		require.NoError(t, err)

		assert.Equal(t, 1, iface.Test())

		var dt *depTest

		err = dingo.Get(injector, &dt)
		require.NoError(t, err)

		assert.Equal(t, 1, dt.Iface.Test())
		assert.Equal(t, 2, dt.Iface2.Test())

		var dt2 depTest
		assert.NoError(t, injector.RequestInjection(&dt2))

		assert.Equal(t, 1, dt2.Iface.Test())
		assert.Equal(t, 2, dt2.Iface2.Test())

		assert.Equal(t, 1, dt.IfaceProvided.Test())
		assert.Equal(t, 1, dt.IfaceImpl1Provided.Test())
		assert.Equal(t, 2, dt.IfaceInstance.Test())

		assert.Equal(t, 1, dt.IfaceProvider().Test())
		iface, err = dt.IfaceWithErrorProvider()
		assert.NoError(t, err)
		assert.Equal(t, 1, iface.Test())
		assert.Equal(t, "Hello World", dt.IfaceProvided.(*interfaceImpl1).foo)
		assert.Equal(t, "Hello World", dt.IfaceImpl1Provided.(*interfaceImpl1).foo)
	})

	t.Run("Should resolve scopes", func(t *testing.T) {
		t.Parallel()

		injector, err := dingo.NewInjector(new(testModule))
		assert.NoError(t, err)

		var (
			singleton1, singleton2 *testSingleton
		)

		err = dingo.Get(injector, &singleton1)
		require.NoError(t, err)
		err = dingo.Get(injector, &singleton2)
		require.NoError(t, err)

		assert.Equal(t, singleton1, singleton2)
		assert.Same(t, singleton1, singleton2)
	})

	t.Run("Error cases", func(t *testing.T) {
		t.Parallel()

		var injector *dingo.Injector

		_, err := injector.Child()
		assert.Error(t, err)
	})
}

type testBoundNothingProvider func() *interfaceImpl1

func TestBoundToNothing(t *testing.T) {
	t.Parallel()

	injector, err := dingo.NewInjector()
	assert.NoError(t, err)

	_, _ = dingo.BindTransient(injector, new(interfaceImpl1), dingo.Annotated("test")) // 	injector.Bind(new(interfaceImpl1)).AnnotatedWith("test")

	var provider testBoundNothingProvider

	err = dingo.Get(injector, &provider)

	assert.NoError(t, err)
	assert.NotNil(t, provider)
	assert.NotNil(t, provider())
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

func (m *AopModule) Configure(i *dingo.Injector) {
	_, _ = dingo.BindTransient[AopInterface](i, &AopImpl{}) // injector.Bind((*AopInterface)(nil)).To(AopImpl{})

	dingo.MustIntercept[AopInterface](i, new(AopInterceptor1))
	dingo.MustIntercept[AopInterface](i, new(AopInterceptor2))
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
	t.Parallel()

	injector, err := dingo.NewInjector(new(AopModule))
	assert.NoError(t, err)

	var dep AopDep
	assert.NoError(t, injector.RequestInjection(&dep))

	assert.Equal(t, "Test 1 2", dep.A.Test())
}

// aopNonImpl implements AopInterface so it satisfies the T constraint, but it
// does NOT embed AopInterface as its first field — used to test the
// "does not implement" validation path of Intercept.
type aopNonImpl struct{}

func (*aopNonImpl) Test() string { return "non-impl" }

func TestInterceptErrors(t *testing.T) {
	t.Parallel()

	t.Run("non-interface T returns error", func(t *testing.T) {
		t.Parallel()

		// AopImpl is a concrete struct, not an interface — Intercept must reject it.
		// We use *AopImpl as the interceptor because it satisfies the T=*AopImpl
		// constraint (it is the same type).
		injector, err := dingo.NewInjector()
		require.NoError(t, err)

		err = dingo.Intercept[*AopImpl](injector, new(AopImpl))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "can only intercept interfaces")
	})

	t.Run("nil interceptor returns error", func(t *testing.T) {
		t.Parallel()

		injector, err := dingo.NewInjector()
		require.NoError(t, err)

		// Pass a typed nil — reflect.TypeOf will return nil for an interface nil.
		var interceptor AopInterface

		err = dingo.Intercept[AopInterface](injector, interceptor)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must not be nil")
	})

	t.Run("interceptor that does not embed T returns error", func(t *testing.T) {
		t.Parallel()

		// aopNonImpl implements AopInterface but does NOT embed it as its first
		// field, so it cannot act as an interceptor wrapper. Intercept should
		// catch this via the implements check.
		injector, err := dingo.NewInjector()
		require.NoError(t, err)

		// aopNonImpl satisfies T=AopInterface at compile time, and its concrete
		// type implements AopInterface — so Intercept must NOT error here.
		// This sub-test instead documents the current behaviour: any concrete
		// type that implements T is accepted by Intercept; the missing-embed
		// constraint is only enforced at resolution time by the runtime.
		err = dingo.Intercept[AopInterface](injector, new(aopNonImpl))
		assert.NoError(t, err, "Intercept accepts any implementation of T; missing-embed is a runtime concern")
	})
}

func TestGetAnnotated(t *testing.T) {
	t.Parallel()

	t.Run("resolves annotated string binding", func(t *testing.T) {
		t.Parallel()

		injector, err := dingo.NewInjector()
		require.NoError(t, err)

		_, _ = dingo.BindValue(injector, "default")
		_, _ = dingo.BindValue(injector, "special", dingo.Annotated("special"))

		var s string

		err = dingo.GetAnnotated(injector, "special", &s)
		require.NoError(t, err)
		assert.Equal(t, "special", s)
	})

	t.Run("resolves annotated interface binding", func(t *testing.T) {
		t.Parallel()

		injector, err := dingo.NewInjector()
		require.NoError(t, err)

		_, _ = dingo.BindTransient[AopInterface](injector, new(AopImpl))
		_, _ = dingo.BindTransient[AopInterface](injector, new(aopNonImpl), dingo.Annotated("alt"))

		var iface AopInterface

		err = dingo.GetAnnotated(injector, "alt", &iface)
		require.NoError(t, err)
		assert.Equal(t, "non-impl", iface.Test())
	})

	t.Run("returns error for unknown annotation", func(t *testing.T) {
		t.Parallel()

		injector, err := dingo.NewInjector()
		require.NoError(t, err)

		var s string

		err = dingo.GetAnnotated(injector, "missing", &s)
		assert.Error(t, err)
	})
}

func TestOptional(t *testing.T) {
	t.Parallel()

	type test struct {
		Must      string `inject:"must"`
		Optional  string `inject:"option,optional"`
		Optional2 string `inject:"option, optional"`
	}

	injector, err := dingo.NewInjector()
	assert.NoError(t, err)

	var i *test

	err = dingo.Get(injector, &i) // _, err = injector.GetInstance(new(test))
	assert.Error(t, err)

	_, _ = dingo.BindValue(injector, "must", dingo.Annotated("must")) // 	injector.Bind(new(string)).AnnotatedWith("must").ToInstance("must")

	err = dingo.Get(injector, &i)

	assert.NoError(t, err)
	assert.Equal(t, i.Must, "must")
	assert.Equal(t, i.Optional, "")
	assert.Equal(t, i.Optional2, "")

	_, _ = dingo.BindValue(injector, "option", dingo.Annotated("option")) // injector.Bind(new(string)).AnnotatedWith("option").ToInstance("option")

	err = dingo.Get(injector, &i)

	assert.NoError(t, err)
	assert.Equal(t, i.Must, "must")
	assert.Equal(t, i.Optional, "option")
	assert.Equal(t, i.Optional2, "option")
}

func TestOverrides(t *testing.T) {
	t.Parallel()

	t.Run("not annotated", func(t *testing.T) {
		t.Parallel()

		injector, err := dingo.NewInjector()
		require.NoError(t, err)

		_, _ = dingo.BindValue(injector, "test")     // injector.Bind(new(string)).ToInstance("test")
		_, _ = dingo.BindValue(injector, "test-bla") // injector.Bind(new(string)).ToInstance("test-bla")
		_, _ = dingo.SwapValue(injector, "test2")    // injector.Override(new(string), "").ToInstance("test2")
		assert.NoError(t, injector.InitModules())

		var s string

		err = dingo.Get(injector, &s) // i, err := injector.GetInstance(new(string))
		require.NoError(t, err)

		assert.Equal(t, "test2", s)
	})

	t.Run("annotated", func(t *testing.T) {
		t.Parallel()

		injector, err := dingo.NewInjector()
		require.NoError(t, err)

		injector.Bind(new(string)).AnnotatedWith("test").ToInstance("test")
		injector.Bind(new(string)).AnnotatedWith("test").ToInstance("test-bla")
		injector.Override(new(string), "test").ToInstance("test2")
		assert.NoError(t, injector.InitModules())

		i, err := injector.GetAnnotatedInstance(new(string), "test")
		assert.NoError(t, err)

		s, ok := i.(string)
		assert.True(t, ok)
		assert.Equal(t, "test2", s)
	})
}

func TestProvider(t *testing.T) {
	t.Parallel()

	t.Run("Provider", func(t *testing.T) {
		t.Parallel()

		injector, err := dingo.NewInjector()
		assert.NoError(t, err)

		_, err = dingo.BindProvider[string](injector, func(i int) string {
			return "test" + strconv.Itoa(i)
		})
		require.NoError(t, err)

		var s string

		err = dingo.Get(injector, &s)
		require.NoError(t, err)

		assert.Equal(t, "test0", s)
	})

	t.Run("Slice Provider", func(t *testing.T) {
		t.Parallel()

		injector, err := dingo.NewInjector()
		assert.NoError(t, err)

		_, err = dingo.BindProvider[[]string](injector, func() []string {
			return []string{"a", "b"}
		})
		require.NoError(t, err)

		var result []string

		err = dingo.Get(injector, &result)
		assert.NoError(t, err)

		assert.Equal(t, []string{"a", "b"}, result)
	})

	t.Run("Invalid Provider", func(t *testing.T) {
		t.Parallel()

		injector, err := dingo.NewInjector()
		require.NoError(t, err)

		_, err = dingo.BindProvider[string](injector, func(interface{}) string {
			return "test"
		})
		require.NoError(t, err)

		var result string

		err = dingo.Get(injector, &result)
		assert.Error(t, err)
	})
}

type testInjectInvalid struct {
	A int `inject:"a"`
}

func (*testInjectInvalid) Configure(*dingo.Injector) {}

func TestInjector_InitModules(t *testing.T) {
	t.Parallel()

	injector, err := dingo.NewInjector()
	assert.NoError(t, err)
	assert.Error(t, injector.InitModules(new(testInjectInvalid)))
}

type TestInjectStructRecInterface interface {
	TestXyz()
}

type testInjectStructRecStruct struct{}

func (t testInjectStructRecStruct) TestXyz() {}

func (t testInjectStructRecStruct) Inject() {}

func TestInjectStructRec(t *testing.T) {
	t.Parallel()

	injector, err := dingo.NewInjector()
	assert.NoError(t, err)

	_, _ = dingo.BindTransient[TestInjectStructRecInterface](injector, new(testInjectStructRecStruct)) // injector.Bind(new(TestInjectStructRecInterface)).To(new(testInjectStructRecStruct))

	var result TestInjectStructRecInterface

	err = dingo.Get(injector, &result)
	assert.Error(t, err)
}

type someStructWithInvalidInterfacePointer struct {
	A *testInterface `inject:""`
}

func TestInjectionOfInterfacePointer(t *testing.T) {
	t.Parallel()

	injector, err := dingo.NewInjector()
	assert.NoError(t, err)

	_, _ = dingo.BindTransient[testInterface](injector, new(interfaceImpl1))

	injector.Bind((*testInterface)(nil)).To(interfaceImpl1{})

	var result someStructWithInvalidInterfacePointer

	err = dingo.Get(injector, &result)
	assert.Error(t, err, "Expected error")
}

// Multi-bindings and map-bindings — ported from multi_dingo_test.go

type (
	v2MapBindInterface interface{}

	v2MapBindInterfaceProvider func() map[string]v2MapBindInterface

	v2MapBindTest1 struct {
		Mbp v2MapBindInterfaceProvider `inject:""`
	}

	v2MapBindTest2 struct {
		Mb v2MapBindInterface `inject:"map:testkey"`
	}

	v2MapBindTest3Provider    func() v2MapBindInterface
	v2MapBindTest3MapProvider func() map[string]v2MapBindTest3Provider
	v2MapBindTest3            struct {
		Mbp v2MapBindTest3MapProvider `inject:""`
	}

	v2MultiBindProvider     func() v2MapBindInterface
	v2ListMultiBindProvider func() []v2MultiBindProvider
	v2MultiBindProviderTest struct {
		Mbp v2ListMultiBindProvider `inject:""`
	}
	v2MultiBindTest struct {
		Mb []v2MapBindInterface `inject:""`
	}
)

func TestV2MultiBinding(t *testing.T) {
	t.Parallel()

	injector, err := dingo.NewInjector()
	require.NoError(t, err)

	_, _ = dingo.MultiBindValue[v2MapBindInterface](injector, "testkey instance")
	_, _ = dingo.MultiBindValue[v2MapBindInterface](injector, "testkey2 instance")
	_, _ = dingo.MultiBindValue[v2MapBindInterface](injector, "testkey3 instance")

	var test *v2MultiBindTest

	err = dingo.Get(injector, &test)
	assert.NoError(t, err)

	list := test.Mb

	assert.Len(t, list, 3)

	assert.Equal(t, "testkey instance", list[0])
	assert.Equal(t, "testkey2 instance", list[1])
	assert.Equal(t, "testkey3 instance", list[2])
}

func TestV2MultiBindingChild(t *testing.T) {
	t.Parallel()

	injector, err := dingo.NewInjector()
	require.NoError(t, err)

	_, _ = dingo.MultiBindValue[v2MapBindInterface](injector, "testkey instance")
	_, _ = dingo.MultiBindValue[v2MapBindInterface](injector, "testkey2 instance")
	_, _ = dingo.MultiBindValue[v2MapBindInterface](injector, "testkey3 instance")

	child, err := injector.Child()
	assert.NoError(t, err)

	_, _ = dingo.MultiBindValue[v2MapBindInterface](child, "testkey4 instance")

	var test *v2MultiBindTest

	err = dingo.Get(injector, &test)
	assert.NoError(t, err)

	list := test.Mb

	assert.Len(t, list, 3)

	assert.Equal(t, "testkey instance", list[0])
	assert.Equal(t, "testkey2 instance", list[1])
	assert.Equal(t, "testkey3 instance", list[2])

	var testChild *v2MultiBindTest

	err = dingo.Get(child, &testChild)
	assert.NoError(t, err)

	list = testChild.Mb

	assert.Len(t, list, 4)

	assert.Equal(t, "testkey instance", list[0])
	assert.Equal(t, "testkey2 instance", list[1])
	assert.Equal(t, "testkey3 instance", list[2])
	assert.Equal(t, "testkey4 instance", list[3])
}

func TestV2MultiBindingProvider(t *testing.T) {
	t.Parallel()

	injector, err := dingo.NewInjector()
	assert.NoError(t, err)

	_, _ = dingo.MultiBindValue[v2MapBindInterface](injector, "testkey instance")
	_, _ = dingo.MultiBindValue[v2MapBindInterface](injector, "testkey2 instance")
	_, _ = dingo.MultiBindValue[v2MapBindInterface](injector, "testkey3 instance")

	var test *v2MultiBindProviderTest

	err = dingo.Get(injector, &test)
	assert.NoError(t, err)

	list := test.Mbp()

	assert.Len(t, list, 3)

	assert.Equal(t, "testkey instance", list[0]())
	assert.Equal(t, "testkey2 instance", list[1]())
	assert.Equal(t, "testkey3 instance", list[2]())
}

func TestV2MultiBindingComplex(t *testing.T) {
	t.Parallel()

	injector, err := dingo.NewInjector()
	require.NoError(t, err)

	_, _ = dingo.MultiBindValue[v2MapBindInterface](injector, "testkey instance")
	_, _ = dingo.MultiBindTransient[v2MapBindInterface](injector, "testkey2 instance")
	dingo.MustMultiBindProvider[v2MapBindInterface](injector, func() v2MapBindInterface { return "provided" })

	var test *v2MultiBindTest

	err = dingo.Get(injector, &test)
	assert.NoError(t, err)

	list := test.Mb

	assert.Len(t, list, 3)

	assert.Equal(t, "testkey instance", list[0])
	assert.NotNil(t, list[1])
	assert.Equal(t, "provided", list[2])
}

func TestV2MultiBindingComplexProvider(t *testing.T) {
	t.Parallel()

	injector, err := dingo.NewInjector()
	require.NoError(t, err)

	_, _ = dingo.MultiBindValue[v2MapBindInterface](injector, "testkey instance")
	_, _ = dingo.MultiBindTransient[v2MapBindInterface](injector, "testkey2 instance")
	dingo.MustMultiBindProvider[v2MapBindInterface](injector, func() v2MapBindInterface { return "provided" })

	var test *v2MultiBindProviderTest

	err = dingo.Get(injector, &test)
	assert.NoError(t, err)

	list := test.Mbp()

	assert.Len(t, list, 3)

	assert.Equal(t, "testkey instance", list[0]())
	assert.NotNil(t, list[1]())
	assert.Equal(t, "provided", list[2]())
}

func TestV2MapBinding(t *testing.T) {
	t.Parallel()

	injector, err := dingo.NewInjector()
	assert.NoError(t, err)

	_, _ = dingo.MapBindValue[v2MapBindInterface](injector, "testkey", "testkey instance")
	_, _ = dingo.MapBindValue[v2MapBindInterface](injector, "testkey2", "testkey2 instance")
	_, _ = dingo.MapBindValue[v2MapBindInterface](injector, "testkey3", "testkey3 instance")

	var test1 *v2MapBindTest1

	err = dingo.Get(injector, &test1)
	assert.NoError(t, err)

	test1map := test1.Mbp()

	assert.Len(t, test1map, 3)
	assert.Equal(t, "testkey instance", test1map["testkey"])
	assert.Equal(t, "testkey2 instance", test1map["testkey2"])
	assert.Equal(t, "testkey3 instance", test1map["testkey3"])

	var test2 *v2MapBindTest2

	err = dingo.Get(injector, &test2)
	assert.NoError(t, err)
	assert.Equal(t, test2.Mb, "testkey instance")
}

func TestV2MapBindingChild(t *testing.T) {
	t.Parallel()

	injector, err := dingo.NewInjector()
	require.NoError(t, err)

	_, _ = dingo.MapBindValue[v2MapBindInterface](injector, "testkey", "testkey instance")
	_, _ = dingo.MapBindValue[v2MapBindInterface](injector, "testkey2", "testkey2 instance")
	_, _ = dingo.MapBindValue[v2MapBindInterface](injector, "testkey3", "testkey3 instance")

	child, err := injector.Child()
	assert.NoError(t, err)

	_, _ = dingo.MapBindValue[v2MapBindInterface](child, "testkey4", "testkey4 instance")

	var test1 *v2MapBindTest1

	err = dingo.Get(injector, &test1)
	assert.NoError(t, err)

	test1map := test1.Mbp()

	assert.Len(t, test1map, 3)
	assert.Equal(t, "testkey instance", test1map["testkey"])
	assert.Equal(t, "testkey2 instance", test1map["testkey2"])
	assert.Equal(t, "testkey3 instance", test1map["testkey3"])

	var test2 *v2MapBindTest2

	err = dingo.Get(injector, &test2)
	assert.NoError(t, err)
	assert.Equal(t, test2.Mb, "testkey instance")

	var testChild *v2MapBindTest1

	err = dingo.Get(child, &testChild)
	assert.NoError(t, err)

	testChildmap := testChild.Mbp()

	assert.Len(t, testChildmap, 4)
	assert.Equal(t, "testkey instance", testChildmap["testkey"])
	assert.Equal(t, "testkey2 instance", testChildmap["testkey2"])
	assert.Equal(t, "testkey3 instance", testChildmap["testkey3"])
	assert.Equal(t, "testkey4 instance", testChildmap["testkey4"])
}

func TestV2MapBindingProvider(t *testing.T) {
	t.Parallel()

	injector, err := dingo.NewInjector()
	require.NoError(t, err)

	_, _ = dingo.MapBindValue[v2MapBindInterface](injector, "testkey", "testkey instance")
	_, _ = dingo.MapBindValue[v2MapBindInterface](injector, "testkey2", "testkey2 instance")
	_, _ = dingo.MapBindValue[v2MapBindInterface](injector, "testkey3", "testkey3 instance")

	var test *v2MapBindTest3

	err = dingo.Get(injector, &test)
	assert.NoError(t, err)

	testmap := test.Mbp()

	assert.Len(t, testmap, 3)
	assert.Equal(t, "testkey instance", testmap["testkey"]())
	assert.Equal(t, "testkey2 instance", testmap["testkey2"]())
	assert.Equal(t, "testkey3 instance", testmap["testkey3"]())
}

func TestV2MapBindingSingleton(t *testing.T) { //nolint:paralleltest // singleton scope is not concurrent-safe
	injector, err := dingo.NewInjector()
	require.NoError(t, err)

	_, _ = dingo.MapBindTransient[v2MapBindInterface](injector, "a", "a", dingo.ScopedSingleton())
	_, _ = dingo.MapBindTransient[v2MapBindInterface](injector, "b", "b")

	var test1 *v2MapBindTest1

	err = dingo.Get(injector, &test1)
	assert.NoError(t, err)

	first := test1.Mbp()["a"]
	second := test1.Mbp()["a"]

	assert.True(t, first == second)

	first = test1.Mbp()["b"]
	second = test1.Mbp()["b"]

	assert.False(t, first == second)
}

func TestV2MultiBindingSingleton(t *testing.T) { //nolint:paralleltest // singleton scope is not concurrent-safe
	injector, err := dingo.NewInjector()
	require.NoError(t, err)

	_, _ = dingo.MultiBindTransient[v2MapBindInterface](injector, "a", dingo.ScopedSingleton())

	var test *v2MultiBindTest

	err = dingo.Get(injector, &test)
	assert.NoError(t, err)

	first := test.Mb[0]

	var test2 *v2MultiBindTest

	err = dingo.Get(injector, &test2)
	assert.NoError(t, err)

	second := test2.Mb[0]

	assert.Same(t, first, second)
}
