package dingo_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"flamingo.me/dingo"
)

// These tests pin the current resolution behaviour through the public
// interface, ahead of restructuring the resolution internals. They use the
// public surface only, and each subtest uses its own types so the shared
// Singleton scope cannot bleed between them.

type (
	charLogger interface {
		Log() string
	}

	charLoggerImpl struct {
		Suffix string
	}

	charLogOne   struct{}
	charLogTwo   struct{}
	charLogBase  struct{}
	charLogOther struct{}

	charPlain struct {
		Unused int
	}

	charAnnotated struct {
		Logger charLogger `inject:"special"`
	}

	charMissing struct {
		Logger charLogger `inject:""`
	}

	charOptional struct {
		Logger charLogger `inject:"optional"`
	}

	charMultiSub struct {
		Loggers []charLogger `inject:""`
	}

	charMapSub struct {
		Loggers map[string]charLogger `inject:""`
	}

	charMapKeySub struct {
		Logger charLogger `inject:"map:logger-a"`
	}

	charSvc         struct{ Suffix string }
	charSvcProvider func() *charSvc
	charSvcUser     struct {
		Make charSvcProvider `inject:""`
	}

	charFallibleProvider func() (*charSvc, error)
	charFallibleUser     struct {
		Make charFallibleProvider `inject:""`
	}

	// non-zero size so pointer identity between instances is meaningful
	charSingletonDep  interface{ IsSingletonDep() int }
	charSingletonImpl struct{ marker int }

	charChildDep  interface{ IsChildDep() int }
	charChildImpl struct{ marker int }

	charDelayedRequest struct {
		Logger charLogger `inject:""`
	}
)

func (l *charLoggerImpl) Log() string { return "log" + l.Suffix }
func (charLogOne) Log() string        { return "log-one" }
func (charLogTwo) Log() string        { return "log-two" }
func (charLogBase) Log() string       { return "log-base" }
func (charLogOther) Log() string      { return "log-other" }

func (i charSingletonImpl) IsSingletonDep() int { return i.marker }
func (i charChildImpl) IsChildDep() int         { return i.marker }

// module that binds the logger implementation
type charBindModule struct{}

func (charBindModule) Configure(injector *dingo.Injector) {
	injector.Bind(new(charLogger)).To(charLoggerImpl{})
}

func TestCharacterizationResolution(t *testing.T) {
	t.Parallel()

	t.Run("bind to concrete type", func(t *testing.T) {
		t.Parallel()

		injector, err := dingo.NewInjector()
		require.NoError(t, err)
		injector.Bind(new(charLogger)).To(charLoggerImpl{})

		instance, err := injector.GetInstance(new(charLogger))
		require.NoError(t, err)
		assert.IsType(t, &charLoggerImpl{}, instance)
	})

	t.Run("bind to instance returns the same pointer", func(t *testing.T) {
		t.Parallel()

		injector, err := dingo.NewInjector()
		require.NoError(t, err)

		shared := &charLoggerImpl{Suffix: "-shared"}
		injector.Bind(new(charLogger)).ToInstance(shared)

		instance, err := injector.GetInstance(new(charLogger))
		require.NoError(t, err)
		assert.Same(t, shared, instance)
	})

	t.Run("bind to provider resolves lazily per request", func(t *testing.T) {
		t.Parallel()

		calls := 0
		injector, err := dingo.NewInjector()
		require.NoError(t, err)
		injector.Bind(new(charPlain)).ToProvider(func() *charPlain {
			calls++
			return &charPlain{Unused: calls}
		})

		assert.Equal(t, 0, calls)

		first, err := injector.GetInstance(new(charPlain))
		require.NoError(t, err)
		second, err := injector.GetInstance(new(charPlain))
		require.NoError(t, err)
		assert.Equal(t, 2, calls)
		assert.NotSame(t, first, second)
	})

	t.Run("provider arguments are injected", func(t *testing.T) {
		t.Parallel()

		injector, err := dingo.NewInjector()
		require.NoError(t, err)
		injector.Bind(new(charLogger)).AnnotatedWith("special").ToInstance(&charLoggerImpl{Suffix: "-arg"})

		type dependentProvider func() *charAnnotated

		type dependentUser struct {
			Make dependentProvider `inject:""`
		}

		var user dependentUser
		require.NoError(t, injector.RequestInjection(&user))
		result := user.Make()
		assert.Equal(t, "-arg", result.Logger.Log()[len("log"):])
	})

	t.Run("annotated binding", func(t *testing.T) {
		t.Parallel()

		injector, err := dingo.NewInjector()
		require.NoError(t, err)
		injector.Bind(new(charLogger)).AnnotatedWith("special").To(charLoggerImpl{})

		instance, err := injector.GetAnnotatedInstance(new(charLogger), "special")
		require.NoError(t, err)
		assert.IsType(t, &charLoggerImpl{}, instance)

		var sub charAnnotated
		require.NoError(t, injector.RequestInjection(&sub))
		assert.NotNil(t, sub.Logger)
	})

	t.Run("missing annotation is an error", func(t *testing.T) {
		t.Parallel()

		injector, err := dingo.NewInjector()
		require.NoError(t, err)

		_, err = injector.GetAnnotatedInstance(new(charLogger), "nothing")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "can not automatically create an annotated injection")
	})

	t.Run("unbound interface is an error", func(t *testing.T) {
		t.Parallel()

		injector, err := dingo.NewInjector()
		require.NoError(t, err)

		_, err = injector.GetInstance(new(charMissing))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "can not instantiate interface")
	})

	t.Run("optional unbound dependency stays zero", func(t *testing.T) {
		t.Parallel()

		injector, err := dingo.NewInjector()
		require.NoError(t, err)

		var sub charOptional
		require.NoError(t, injector.RequestInjection(&sub))
		assert.Nil(t, sub.Logger)
	})

	t.Run("plain struct is constructed without bindings", func(t *testing.T) {
		t.Parallel()

		injector, err := dingo.NewInjector()
		require.NoError(t, err)

		instance, err := injector.GetInstance(new(charPlain))
		require.NoError(t, err)
		assert.IsType(t, &charPlain{}, instance)
	})

	t.Run("provider function field", func(t *testing.T) {
		t.Parallel()

		injector, err := dingo.NewInjector()
		require.NoError(t, err)

		var user charSvcUser
		require.NoError(t, injector.RequestInjection(&user))
		assert.NotNil(t, user.Make)
		assert.Equal(t, &charSvc{}, user.Make())
	})

	t.Run("fallible provider function field", func(t *testing.T) {
		t.Parallel()

		injector, err := dingo.NewInjector()
		require.NoError(t, err)

		var user charFallibleUser
		require.NoError(t, injector.RequestInjection(&user))
		value, err := user.Make()
		require.NoError(t, err)
		assert.Equal(t, &charSvc{}, value)
	})
}

func TestCharacterizationCollections(t *testing.T) {
	t.Parallel()

	t.Run("multibinding collects in binding order", func(t *testing.T) {
		t.Parallel()

		injector, err := dingo.NewInjector()
		require.NoError(t, err)
		injector.BindMulti(new(charLogger)).To(charLogOne{})
		injector.BindMulti(new(charLogger)).To(charLogTwo{})

		var sub charMultiSub
		require.NoError(t, injector.RequestInjection(&sub))
		require.Len(t, sub.Loggers, 2)
		assert.Equal(t, "log-one", sub.Loggers[0].Log())
		assert.Equal(t, "log-two", sub.Loggers[1].Log())
	})

	t.Run("multibinding without contributions is empty", func(t *testing.T) {
		t.Parallel()

		injector, err := dingo.NewInjector()
		require.NoError(t, err)

		var sub charMultiSub
		require.NoError(t, injector.RequestInjection(&sub))
		assert.Empty(t, sub.Loggers)
	})

	t.Run("mapbinding injects the whole map", func(t *testing.T) {
		t.Parallel()

		injector, err := dingo.NewInjector()
		require.NoError(t, err)
		injector.BindMap(new(charLogger), "logger-a").To(charLogOne{})
		injector.BindMap(new(charLogger), "logger-b").To(charLogTwo{})

		var sub charMapSub
		require.NoError(t, injector.RequestInjection(&sub))
		require.Len(t, sub.Loggers, 2)
		assert.Equal(t, "log-one", sub.Loggers["logger-a"].Log())
		assert.Equal(t, "log-two", sub.Loggers["logger-b"].Log())
	})

	t.Run("map annotation injects a single entry", func(t *testing.T) {
		t.Parallel()

		injector, err := dingo.NewInjector()
		require.NoError(t, err)
		injector.BindMap(new(charLogger), "logger-a").To(charLogOne{})

		var sub charMapKeySub
		require.NoError(t, injector.RequestInjection(&sub))
		assert.Equal(t, "log-one", sub.Logger.Log())
	})

	t.Run("provider functions in multibindings", func(t *testing.T) {
		t.Parallel()

		type loggerProvider func() charLogger

		type providerSub struct {
			Makers []loggerProvider `inject:""`
		}

		injector, err := dingo.NewInjector()
		require.NoError(t, err)
		injector.BindMulti(new(charLogger)).To(charLogOne{})

		var sub providerSub
		require.NoError(t, injector.RequestInjection(&sub))
		require.Len(t, sub.Makers, 1)
		assert.Equal(t, "log-one", sub.Makers[0]().Log())
	})
}

func TestCharacterizationScopesAndParents(t *testing.T) {
	t.Parallel()

	t.Run("unscoped binding creates a fresh instance per request", func(t *testing.T) {
		t.Parallel()

		injector, err := dingo.NewInjector()
		require.NoError(t, err)
		injector.Bind(new(charSingletonDep)).To(charSingletonImpl{})

		first, err := injector.GetInstance(new(charSingletonDep))
		require.NoError(t, err)
		second, err := injector.GetInstance(new(charSingletonDep))
		require.NoError(t, err)
		assert.NotSame(t, first, second)
	})

	t.Run("singleton returns the same instance", func(t *testing.T) {
		t.Parallel()

		injector, err := dingo.NewInjector()
		require.NoError(t, err)
		injector.Bind(new(charSingletonDep)).To(charSingletonImpl{}).In(dingo.Singleton)

		first, err := injector.GetInstance(new(charSingletonDep))
		require.NoError(t, err)
		second, err := injector.GetInstance(new(charSingletonDep))
		require.NoError(t, err)
		assert.Same(t, first, second)
	})

	t.Run("parent bindings resolve in the child", func(t *testing.T) {
		t.Parallel()

		parent, err := dingo.NewInjector()
		require.NoError(t, err)
		parent.Bind(new(charLogger)).ToInstance(&charLoggerImpl{Suffix: "-parent"})

		child, err := parent.Child()
		require.NoError(t, err)

		instance, err := child.GetInstance(new(charLogger))
		require.NoError(t, err)

		logger, ok := instance.(charLogger)
		require.True(t, ok)
		assert.Equal(t, "log-parent", logger.Log())
	})

	t.Run("child binding shadows the parent binding", func(t *testing.T) {
		t.Parallel()

		parent, err := dingo.NewInjector()
		require.NoError(t, err)
		parent.Bind(new(charLogger)).ToInstance(&charLoggerImpl{Suffix: "-parent"})

		child, err := parent.Child()
		require.NoError(t, err)
		child.Bind(new(charLogger)).ToInstance(&charLoggerImpl{Suffix: "-child"})

		instance, err := child.GetInstance(new(charLogger))
		require.NoError(t, err)

		logger, ok := instance.(charLogger)
		require.True(t, ok)
		assert.Equal(t, "log-child", logger.Log())
	})

	t.Run("child singleton scope is local to the child", func(t *testing.T) {
		t.Parallel()

		parent, err := dingo.NewInjector()
		require.NoError(t, err)
		parent.Bind(new(charChildDep)).To(charChildImpl{}).In(dingo.ChildSingleton)

		parentInstance, err := parent.GetInstance(new(charChildDep))
		require.NoError(t, err)
		child, err := parent.Child()
		require.NoError(t, err)
		childInstance, err := child.GetInstance(new(charChildDep))
		require.NoError(t, err)
		againInChild, err := child.GetInstance(new(charChildDep))
		require.NoError(t, err)

		assert.NotSame(t, parentInstance, childInstance)
		assert.Same(t, childInstance, againInChild)
	})

	t.Run("eager singleton is built during initialization", func(t *testing.T) {
		t.Parallel()

		calls := 0
		module := dingo.ModuleFunc(func(injector *dingo.Injector) {
			injector.Bind(new(charPlain)).ToProvider(func() *charPlain {
				calls++
				return &charPlain{}
			}).AsEagerSingleton()
		})

		_, err := dingo.NewInjector(module)
		require.NoError(t, err)
		assert.Equal(t, 1, calls)
	})
}

func TestCharacterizationInitModules(t *testing.T) {
	t.Parallel()

	t.Run("duplicate bindings are rejected", func(t *testing.T) {
		t.Parallel()

		duplicate := dingo.ModuleFunc(func(injector *dingo.Injector) {
			injector.Bind(new(charLogger)).To(charLogBase{})
			injector.Bind(new(charLogger)).To(charLogTwo{})
		})

		_, err := dingo.NewInjector(duplicate)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already known binding")
	})

	t.Run("identical duplicate bindings are accepted", func(t *testing.T) {
		t.Parallel()

		identical := dingo.ModuleFunc(func(injector *dingo.Injector) {
			injector.Bind(new(charLogger)).To(charLoggerImpl{})
			injector.Bind(new(charLogger)).To(charLoggerImpl{})
		})

		_, err := dingo.NewInjector(identical)
		assert.NoError(t, err)
	})

	t.Run("override replaces the original binding", func(t *testing.T) {
		t.Parallel()

		base := dingo.ModuleFunc(func(injector *dingo.Injector) {
			injector.Bind(new(charLogger)).To(charLogBase{})
		})
		override := dingo.ModuleFunc(func(injector *dingo.Injector) {
			injector.Override(new(charLogger), "").To(charLogOther{})
		})

		injector, err := dingo.NewInjector(base, override)
		require.NoError(t, err)

		instance, err := injector.GetInstance(new(charLogger))
		require.NoError(t, err)

		logger, ok := instance.(charLogger)
		require.True(t, ok)
		assert.Equal(t, "log-other", logger.Log())
	})

	t.Run("override of an unknown annotation creates the binding", func(t *testing.T) {
		t.Parallel()

		// Override binds before it records the override, so the
		// "cannot override unknown binding" error is unreachable through
		// the public interface: an override without an original binding
		// silently creates the annotated binding.
		module := dingo.ModuleFunc(func(injector *dingo.Injector) {
			injector.Override(new(charLogger), "missing").To(charLogOther{})
		})

		injector, err := dingo.NewInjector(module)
		require.NoError(t, err)

		instance, err := injector.GetAnnotatedInstance(new(charLogger), "missing")
		require.NoError(t, err)

		logger, ok := instance.(charLogger)
		require.True(t, ok)
		assert.Equal(t, "log-other", logger.Log())
	})

	t.Run("request during init is deferred until modules are configured", func(t *testing.T) {
		t.Parallel()

		requested := &charDelayedRequest{}
		requesting := dingo.ModuleFunc(func(injector *dingo.Injector) {
			require.NoError(t, injector.RequestInjection(requested))
		})

		_, err := dingo.NewInjector(requesting, charBindModule{})
		require.NoError(t, err)
		assert.NotNil(t, requested.Logger)
	})
}
