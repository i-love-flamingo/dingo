package releasediff_test

import (
	"fmt"
	"sort"
	"strings"
	"sync/atomic"

	"flamingo.me/dingo"
)

// area stands in for Flamingo's config.Area, which binds itself.
type area struct{ name string }

// newArea mirrors Flamingo's Area.GetInitializedInjector: a child of the
// parent area's injector (or a new injector for the root area), eager
// singletons off, the area bound as an instance, each config value bound as
// a "config:<key>" instance (integral floats also as int64 and int), then
// InitModules.
func newArea(parent *dingo.Injector, cfg map[string]any, modules ...dingo.Module) (*dingo.Injector, error) {
	var (
		injector *dingo.Injector
		err      error
	)

	if parent != nil {
		injector, err = parent.Child()
	} else {
		injector, err = dingo.NewInjector()
	}

	if err != nil {
		return nil, err
	}

	injector.SetBuildEagerSingletons(false)
	injector.Bind(area{}).ToInstance(&area{name: fmt.Sprint(cfg["site"])})

	keys := make([]string, 0, len(cfg))
	for k := range cfg {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	for _, k := range keys {
		v := cfg[k]
		injector.Bind(v).AnnotatedWith("config:" + k).ToInstance(v)

		if vf, ok := v.(float64); ok && vf == float64(int64(vf)) {
			injector.Bind(new(int64)).AnnotatedWith("config:" + k).ToInstance(int64(vf))
			injector.Bind(new(int)).AnnotatedWith("config:" + k).ToInstance(int(int64(vf)))
		}
	}

	return injector, injector.InitModules(modules...)
}

func mustArea(parent *dingo.Injector, cfg map[string]any, modules ...dingo.Module) *dingo.Injector {
	injector, err := newArea(parent, cfg, modules...)
	if err != nil {
		panic(fmt.Sprintf("area setup: %v", err))
	}

	return injector
}

type namedArea struct {
	name     string
	injector *dingo.Injector
}

// Handler is the interface most probes bind, like a Flamingo service port.
type Handler interface{ Name() string }

type (
	HandlerProvider    func() Handler
	HandlerErrProvider func() (Handler, error)
)

type namedHandler struct{ name string }

func (h *namedHandler) Name() string { return h.name }

// siteHandler reads its area's config, like most Flamingo services.
type siteHandler struct {
	Site string `inject:"config:site"`
}

func (h *siteHandler) Name() string { return "site:" + h.Site }

// constructions counts countedHandler instances, so probes can tell new
// instances from reused ones.
var constructions atomic.Int64

type countedHandler struct{ id int64 }

func (c *countedHandler) Inject()      { c.id = constructions.Add(1) }
func (c *countedHandler) Name() string { return fmt.Sprintf("counted#%d", c.id) }

// Service is a second interface with a real and a fake implementation, like
// the adapters Flamingo Commerce swaps for fakes.
type Service interface{ Name() string }

type realService struct{}

func (*realService) Name() string { return "real" }

type fakeService struct{}

func (*fakeService) Name() string { return "fake" }

// Target is a plain struct, bound by value and by pointer.
type Target struct{ N string }

func (t *Target) Name() string { return "target:" + t.N }

type Leaf struct{ X string }

type Mid struct {
	L *Leaf `inject:""`
}

// handlerNames prints Handler maps and slices in a stable order.
func handlerNames(v any, err error) string {
	if err != nil {
		return result(nil, err)
	}

	switch handlers := v.(type) {
	case map[string]Handler:
		keys := make([]string, 0, len(handlers))
		for k := range handlers {
			keys = append(keys, k)
		}

		sort.Strings(keys)

		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, k+":"+handlerName(handlers[k]))
		}

		return "{" + strings.Join(parts, ",") + "}"
	case []Handler:
		parts := make([]string, 0, len(handlers))
		for _, h := range handlers {
			parts = append(parts, handlerName(h))
		}

		return "[" + strings.Join(parts, ",") + "]"
	}

	return show(v)
}

func handlerName(h Handler) string {
	if h == nil {
		return "nil"
	}

	return catch(h.Name)
}

// mapAreas builds the area tree most map and multibinding probes use:
// root binds map keys a, b and site and two multibindings; de overrides key b,
// adds key c and a multibinding; fr binds only config; de/sub binds nothing.
func mapAreas() []namedArea {
	rootModule := dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.BindMap(new(Handler), "a").ToInstance(&namedHandler{"root-a"})
		injector.BindMap(new(Handler), "b").ToInstance(&namedHandler{"root-b"})
		injector.BindMap(new(Handler), "site").To(siteHandler{})
		injector.BindMulti(new(Handler)).ToInstance(&namedHandler{"root-m1"})
		injector.BindMulti(new(Handler)).To(siteHandler{})
	})
	deModule := dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.BindMap(new(Handler), "b").ToInstance(&namedHandler{"de-b"})
		injector.BindMap(new(Handler), "c").ToInstance(&namedHandler{"de-c"})
		injector.BindMulti(new(Handler)).ToInstance(&namedHandler{"de-m3"})
	})

	root := mustArea(nil, map[string]any{"site": "root", "only.root": "R", "num": float64(7)}, rootModule)
	de := mustArea(root, map[string]any{"site": "de"}, deModule)
	fr := mustArea(root, map[string]any{"site": "fr"})
	deSub := mustArea(de, map[string]any{"site": "de-sub"})

	return []namedArea{{"root", root}, {"de", de}, {"fr", fr}, {"de/sub", deSub}}
}
