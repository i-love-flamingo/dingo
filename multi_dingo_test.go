package dingo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type (
	mapBindInterface interface{}

	mapBindInterfaceProvider func() map[string]mapBindInterface

	mapBindTest1 struct {
		Mbp mapBindInterfaceProvider `inject:""`
	}

	mapBindTest2 struct {
		Mb mapBindInterface `inject:"map:testkey"`
	}

	mapBindTest3Provider    func() mapBindInterface
	mapBindTest3MapProvider func() map[string]mapBindTest3Provider
	mapBindTest3            struct {
		Mbp mapBindTest3MapProvider `inject:""`
	}

	multiBindProvider     func() mapBindInterface
	listmultiBindProvider func() []multiBindProvider
	multiBindProviderTest struct {
		Mbp listmultiBindProvider `inject:""`
	}
	multiBindTest struct {
		Mb []mapBindInterface `inject:""`
	}
)

func TestMultiBinding(t *testing.T) {
	injector, err := NewInjector()
	assert.NoError(t, err)

	injector.BindMulti((*mapBindInterface)(nil)).ToInstance("testkey instance")
	injector.BindMulti((*mapBindInterface)(nil)).ToInstance("testkey2 instance")
	injector.BindMulti((*mapBindInterface)(nil)).ToInstance("testkey3 instance")

	i, err := injector.GetInstance(&multiBindTest{})
	assert.NoError(t, err)
	test := i.(*multiBindTest)
	list := test.Mb

	assert.Len(t, list, 3)

	assert.Equal(t, "testkey instance", list[0])
	assert.Equal(t, "testkey2 instance", list[1])
	assert.Equal(t, "testkey3 instance", list[2])
}

func TestMultiBindingChild(t *testing.T) {
	injector, err := NewInjector()
	assert.NoError(t, err)

	injector.BindMulti((*mapBindInterface)(nil)).ToInstance("testkey instance")
	injector.BindMulti((*mapBindInterface)(nil)).ToInstance("testkey2 instance")
	injector.BindMulti((*mapBindInterface)(nil)).ToInstance("testkey3 instance")

	child, err := injector.Child()
	assert.NoError(t, err)
	child.BindMulti((*mapBindInterface)(nil)).ToInstance("testkey4 instance")

	i, err := injector.GetInstance(&multiBindTest{})
	assert.NoError(t, err)
	test := i.(*multiBindTest)
	list := test.Mb

	assert.Len(t, list, 3)

	assert.Equal(t, "testkey instance", list[0])
	assert.Equal(t, "testkey2 instance", list[1])
	assert.Equal(t, "testkey3 instance", list[2])

	i, err = child.GetInstance(&multiBindTest{})
	assert.NoError(t, err)
	test = i.(*multiBindTest)
	list = test.Mb

	assert.Len(t, list, 4)

	assert.Equal(t, "testkey instance", list[0])
	assert.Equal(t, "testkey2 instance", list[1])
	assert.Equal(t, "testkey3 instance", list[2])
	assert.Equal(t, "testkey4 instance", list[3])
}

func TestMultiBindingProvider(t *testing.T) {
	injector, err := NewInjector()
	assert.NoError(t, err)

	injector.BindMulti((*mapBindInterface)(nil)).ToInstance("testkey instance")
	injector.BindMulti((*mapBindInterface)(nil)).ToInstance("testkey2 instance")
	injector.BindMulti((*mapBindInterface)(nil)).ToInstance("testkey3 instance")

	i, err := injector.GetInstance(&multiBindProviderTest{})
	assert.NoError(t, err)
	test := i.(*multiBindProviderTest)
	list := test.Mbp()

	assert.Len(t, list, 3)

	assert.Equal(t, "testkey instance", list[0]())
	assert.Equal(t, "testkey2 instance", list[1]())
	assert.Equal(t, "testkey3 instance", list[2]())
}

func TestMultiBindingComplex(t *testing.T) {
	injector, err := NewInjector()
	assert.NoError(t, err)

	injector.BindMulti((*mapBindInterface)(nil)).ToInstance("testkey instance")
	injector.BindMulti((*mapBindInterface)(nil)).To("testkey2 instance")
	injector.BindMulti((*mapBindInterface)(nil)).ToProvider(func() mapBindInterface { return "provided" })

	i, err := injector.GetInstance(&multiBindTest{})
	assert.NoError(t, err)
	test := i.(*multiBindTest)
	list := test.Mb

	assert.Len(t, list, 3)

	assert.Equal(t, "testkey instance", list[0])
	assert.NotNil(t, list[1])
	assert.Equal(t, "provided", list[2])
}

func TestMultiBindingComplexProvider(t *testing.T) {
	injector, err := NewInjector()
	assert.NoError(t, err)

	injector.BindMulti((*mapBindInterface)(nil)).ToInstance("testkey instance")
	injector.BindMulti((*mapBindInterface)(nil)).To("testkey2 instance")
	injector.BindMulti((*mapBindInterface)(nil)).ToProvider(func() mapBindInterface { return "provided" })

	i, err := injector.GetInstance(&multiBindProviderTest{})
	assert.NoError(t, err)
	test := i.(*multiBindProviderTest)
	list := test.Mbp()

	assert.Len(t, list, 3)

	assert.Equal(t, "testkey instance", list[0]())
	assert.NotNil(t, list[1]())
	assert.Equal(t, "provided", list[2]())
}

func TestMapBinding(t *testing.T) {
	injector, err := NewInjector()
	assert.NoError(t, err)

	injector.BindMap((*mapBindInterface)(nil), "testkey").ToInstance("testkey instance")
	injector.BindMap((*mapBindInterface)(nil), "testkey2").ToInstance("testkey2 instance")
	injector.BindMap((*mapBindInterface)(nil), "testkey3").ToInstance("testkey3 instance")

	i, err := injector.GetInstance(&mapBindTest1{})
	assert.NoError(t, err)
	test1 := i.(*mapBindTest1)
	test1map := test1.Mbp()

	assert.Len(t, test1map, 3)
	assert.Equal(t, "testkey instance", test1map["testkey"])
	assert.Equal(t, "testkey2 instance", test1map["testkey2"])
	assert.Equal(t, "testkey3 instance", test1map["testkey3"])

	i, err = injector.GetInstance(&mapBindTest2{})
	assert.NoError(t, err)
	test2 := i.(*mapBindTest2)
	assert.Equal(t, test2.Mb, "testkey instance")
}

func TestMapBindingChild(t *testing.T) {
	injector, err := NewInjector()
	assert.NoError(t, err)

	injector.BindMap((*mapBindInterface)(nil), "testkey").ToInstance("testkey instance")
	injector.BindMap((*mapBindInterface)(nil), "testkey2").ToInstance("testkey2 instance")
	injector.BindMap((*mapBindInterface)(nil), "testkey3").ToInstance("testkey3 instance")

	child, err := injector.Child()
	assert.NoError(t, err)
	child.BindMap((*mapBindInterface)(nil), "testkey4").ToInstance("testkey4 instance")

	i, err := injector.GetInstance(&mapBindTest1{})
	assert.NoError(t, err)
	test1 := i.(*mapBindTest1)
	test1map := test1.Mbp()

	assert.Len(t, test1map, 3)
	assert.Equal(t, "testkey instance", test1map["testkey"])
	assert.Equal(t, "testkey2 instance", test1map["testkey2"])
	assert.Equal(t, "testkey3 instance", test1map["testkey3"])

	i, err = injector.GetInstance(&mapBindTest2{})
	assert.NoError(t, err)
	test2 := i.(*mapBindTest2)
	assert.Equal(t, test2.Mb, "testkey instance")

	i, err = child.GetInstance(&mapBindTest1{})
	assert.NoError(t, err)
	testChild := i.(*mapBindTest1)
	testChildmap := testChild.Mbp()

	assert.Len(t, testChildmap, 4)
	assert.Equal(t, "testkey instance", testChildmap["testkey"])
	assert.Equal(t, "testkey2 instance", testChildmap["testkey2"])
	assert.Equal(t, "testkey3 instance", testChildmap["testkey3"])
	assert.Equal(t, "testkey4 instance", testChildmap["testkey4"])
}

func TestMapBindingProvider(t *testing.T) {
	injector, err := NewInjector()
	assert.NoError(t, err)

	injector.BindMap((*mapBindInterface)(nil), "testkey").ToInstance("testkey instance")
	injector.BindMap((*mapBindInterface)(nil), "testkey2").ToInstance("testkey2 instance")
	injector.BindMap((*mapBindInterface)(nil), "testkey3").ToInstance("testkey3 instance")

	i, err := injector.GetInstance(&mapBindTest3{})
	assert.NoError(t, err)
	test := i.(*mapBindTest3)
	testmap := test.Mbp()

	assert.Len(t, testmap, 3)
	assert.Equal(t, "testkey instance", testmap["testkey"]())
	assert.Equal(t, "testkey2 instance", testmap["testkey2"]())
	assert.Equal(t, "testkey3 instance", testmap["testkey3"]())
}
