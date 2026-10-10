package dingo

import (
	"errors"
	"fmt"
	"reflect"
)

const (
	// INIT state
	INIT = iota
	// DEFAULT state
	DEFAULT
)

var (
	ErrInitModules           = errors.New("initialization of modules failed")
	ErrInvalidInjectReceiver = errors.New("usage of 'Inject' method with struct receiver is not allowed")
	errPointersToInterface   = errors.New(" Do not use pointers to interface")

	// defaults seed every new injector with the tracing mode that the deprecated
	// package-level Enable* functions set; they are read once, at construction.
	defaultCircularTracing  = false
	defaultInjectionTracing = false
)

// EnableCircularTracing activates dingo's trace feature to find circular dependencies.
// This is super expensive (memory wise), so it should only be used for debugging purposes.
//
// Deprecated: sets the default for injectors created after this call; it does not
// affect existing injectors. Prefer Injector.EnableCircularTracing, which scopes the
// decision to one injector. Removal is planned for the next minor version.
func EnableCircularTracing() {
	defaultCircularTracing = true
}

// EnableInjectionTracing activates verbose logging of everything dingo injects.
//
// Deprecated: sets the default for injectors created after this call; it does not
// affect existing injectors. Prefer Injector.EnableInjectionTracing, which scopes the
// decision to one injector. Removal is planned for the next minor version.
func EnableInjectionTracing() {
	defaultInjectionTracing = true
}

type (
	// Injector defines bindings and multibindings
	// it is possible to have a parent-injector, which can be asked if no resolution is available
	Injector struct {
		bindings             map[reflect.Type][]*Binding          // list of available bindings for a concrete type
		multibindings        map[reflect.Type][]*Binding          // list of multi-bindings for a concrete type
		mapbindings          map[reflect.Type]map[string]*Binding // list of map-bindings for a concrete type
		interceptor          map[reflect.Type][]reflect.Type      // list of interceptors for a type
		overrides            []*override                          // list of overrides for a binding
		parent               *Injector                            // parent injector reference
		scopes               map[reflect.Type]Scope               // scope-bindings
		stage                uint                                 // current stage
		delayed              []interface{}                        // delayed bindings
		buildEagerSingletons bool                                 // whether to build singletons
		circularTracing      bool                                 // circular-dependency tracing enabled
		injectionTracing     bool                                 // verbose injection logging enabled
	}

	// overrides are evaluated lazy, so they are scheduled here
	override struct {
		typ           reflect.Type
		annotatedWith string
		binding       *Binding
	}

	circularTraceEntry struct {
		typ        reflect.Type
		annotation string
	}
)

// NewInjector builds up a new Injector out of a list of Modules
func NewInjector(modules ...Module) (*Injector, error) {
	injector := &Injector{
		bindings:             make(map[reflect.Type][]*Binding),
		multibindings:        make(map[reflect.Type][]*Binding),
		mapbindings:          make(map[reflect.Type]map[string]*Binding),
		interceptor:          make(map[reflect.Type][]reflect.Type),
		scopes:               make(map[reflect.Type]Scope),
		stage:                DEFAULT,
		buildEagerSingletons: true,
		circularTracing:      defaultCircularTracing,
		injectionTracing:     defaultInjectionTracing,
	}

	// bind current injector
	injector.Bind(Injector{}).ToInstance(injector)

	// bind default scopes
	injector.BindScope(Singleton)
	injector.BindScope(ChildSingleton)

	// init current modules
	return injector, injector.InitModules(modules...)
}

// Child derives a child injector with a new ChildSingletonScope
func (injector *Injector) Child() (*Injector, error) {
	if injector == nil {
		return nil, errors.New("can not create a child of an uninitialized injector")
	}

	newInjector, err := NewInjector()
	if err != nil {
		return nil, err
	}

	newInjector.parent = injector
	newInjector.circularTracing = injector.circularTracing
	newInjector.injectionTracing = injector.injectionTracing
	newInjector.Bind(Injector{}).ToInstance(newInjector)
	newInjector.BindScope(NewChildSingletonScope()) // bind a new child-singleton

	return newInjector, nil
}

// InitModules initializes the injector with the given modules
func (injector *Injector) InitModules(modules ...Module) error {
	injector.stage = INIT

	mg := newModuleGraph()

	err := mg.Add(modules...)
	if err != nil {
		return fmt.Errorf("%w: failed adding modules to the graph: %w", ErrInitModules, err)
	}

	modules, err = mg.Sort()
	if err != nil {
		return fmt.Errorf("%w: failed sorting modules: %w", ErrInitModules, err)
	}

	return injector.initModules(modules)
}

// InitExpandedModules initializes the supplied modules in order without calling
// Depends. It keeps the first occurrence of each module identity and does not
// modify the input slice. Use it to initialize an exact ExpandModules snapshot.
func (injector *Injector) InitExpandedModules(modules ...Module) error {
	unique := make([]Module, 0, len(modules))
	seen := make(map[moduleKey]struct{}, len(modules))

	for _, module := range modules {
		key := moduleKeyOf(module)
		if _, ok := seen[key]; ok {
			continue
		}

		seen[key] = struct{}{}

		unique = append(unique, module)
	}

	return injector.initModules(unique)
}

//nolint:gocognit,cyclop // shared finalization preserves the existing initialization behavior
func (injector *Injector) initModules(modules []Module) error {
	injector.stage = INIT

	for _, module := range modules {
		if err := injector.requestInjection(module, injector.newTrace()); err != nil {
			erroredModule := reflect.TypeOf(module).Elem()
			return fmt.Errorf("initmodules: injection into %q failed: %w", erroredModule.PkgPath()+"."+erroredModule.Name(), err)
		}
		module.Configure(injector)
	}

	// evaluate overrides when modules were loaded
	for _, override := range injector.overrides {
		bindtype := override.typ
		if bindtype.Kind() == reflect.Ptr {
			bindtype = bindtype.Elem()
		}
		if bindings, ok := injector.bindings[bindtype]; ok && len(bindings) > 0 {
			for i, binding := range bindings {
				if binding.annotatedWith == override.annotatedWith {
					injector.bindings[bindtype][i] = override.binding
				}
			}
			continue
		}
		return fmt.Errorf("cannot override unknown binding %q (annotated with %q)", override.typ.String(), override.annotatedWith) // todo ok?
	}

	// make sure there are no duplicated bindings
	for typ, bindings := range injector.bindings {
		known := make(map[string]*Binding)
		for _, binding := range bindings {
			if known, ok := known[binding.annotatedWith]; ok && !known.equal(binding) {
				var knownBinding, duplicateBinding string
				if known.to != nil {
					knownBinding = fmt.Sprintf("%#v%#v", known.to.PkgPath(), known.to.Name())
				}
				if binding.to != nil {
					duplicateBinding = fmt.Sprintf("%#v%#v", binding.to.PkgPath(), binding.to.Name())
				}
				return fmt.Errorf("already known binding for %q with annotation %q | Known binding: %q Try %q", typ, binding.annotatedWith, knownBinding, duplicateBinding)
			}
			known[binding.annotatedWith] = binding
		}
	}

	injector.stage = DEFAULT

	// continue with delayed injections
	for _, object := range injector.delayed {
		if err := injector.requestInjection(object, injector.newTrace()); err != nil {
			return err
		}
	}

	injector.delayed = nil

	// build eager singletons
	if !injector.buildEagerSingletons {
		return nil
	}

	return injector.BuildEagerSingletons(false)
}

// SetBuildEagerSingletons can be used to disable or enable building of eager singletons during InitModules
func (injector *Injector) SetBuildEagerSingletons(build bool) {
	injector.buildEagerSingletons = build
}

// EnableCircularTracing activates tracing of circular dependencies for this injector.
// This is super expensive (memory wise), so it should only be used for debugging purposes.
func (injector *Injector) EnableCircularTracing() {
	injector.circularTracing = true
}

// EnableInjectionTracing activates verbose logging of everything this injector injects.
func (injector *Injector) EnableInjectionTracing() {
	injector.injectionTracing = true
}

// newTrace seeds a circular-dependency trace: empty when tracing is on, nil when off.
func (injector *Injector) newTrace() []circularTraceEntry {
	if injector.circularTracing {
		return make([]circularTraceEntry, 0)
	}

	return nil
}

// BuildEagerSingletons requests one instance of each singleton, optional letting the parent injector(s) do the same
func (injector *Injector) BuildEagerSingletons(includeParent bool) error {
	for _, bindings := range injector.bindings {
		for _, binding := range bindings {
			if binding.eager {
				if _, err := injector.getInstance(binding.typeof, binding.annotatedWith, injector.newTrace()); err != nil {
					return fmt.Errorf("initmodules: loading eager singletons: %w", err)
				}
			}
		}
	}
	if includeParent && injector.parent != nil {
		return injector.parent.BuildEagerSingletons(includeParent)
	}

	return nil
}

// GetInstance creates a new instance of what was requested
func (injector *Injector) GetInstance(of interface{}) (interface{}, error) {
	i, err := injector.getInstance(of, "", injector.newTrace())
	if err != nil {
		return nil, err
	}

	return i.Interface(), nil
}

// GetAnnotatedInstance creates a new instance of what was requested with the given annotation
func (injector *Injector) GetAnnotatedInstance(of interface{}, annotatedWith string) (interface{}, error) {
	i, err := injector.getInstance(of, annotatedWith, injector.newTrace())
	if err != nil {
		return nil, err
	}

	return i.Interface(), nil
}

// BindMulti binds multiple concrete types to the same abstract type / interface
func (injector *Injector) BindMulti(what interface{}) *Binding {
	bindtype := reflect.TypeOf(what)
	if bindtype.Kind() == reflect.Ptr {
		bindtype = bindtype.Elem()
	}
	binding := new(Binding)
	binding.typeof = bindtype
	imb := injector.multibindings[bindtype]
	imb = append(imb, binding)
	injector.multibindings[bindtype] = imb

	return binding
}

// BindMap does a registry-like map-based binding, like BindMulti
func (injector *Injector) BindMap(what interface{}, key string) *Binding {
	bindtype := reflect.TypeOf(what)
	if bindtype.Kind() == reflect.Ptr {
		bindtype = bindtype.Elem()
	}
	binding := new(Binding)
	binding.typeof = bindtype
	bindingMap := injector.mapbindings[bindtype]
	if bindingMap == nil {
		bindingMap = make(map[string]*Binding)
	}
	bindingMap[key] = binding
	injector.mapbindings[bindtype] = bindingMap

	return binding
}

// BindInterceptor intercepts to interface with interceptor
func (injector *Injector) BindInterceptor(to, interceptor interface{}) {
	totype := reflect.TypeOf(to)
	if totype.Kind() == reflect.Ptr {
		totype = totype.Elem()
	}
	if totype.Kind() != reflect.Interface {
		panic("can only intercept interfaces " + fmt.Sprintf("%v", to))
	}
	m := injector.interceptor[totype]
	m = append(m, reflect.TypeOf(interceptor))
	injector.interceptor[totype] = m
}

// BindScope binds a scope to be aware of
func (injector *Injector) BindScope(s Scope) {
	injector.scopes[reflect.TypeOf(s)] = s
}

// Bind creates a new binding for an abstract type / interface
// Use the syntax
//
//	injector.Bind((*Interface)(nil))
//
// To specify the interface (cast it to a pointer to a nil of the type Interface)
func (injector *Injector) Bind(what interface{}) *Binding {
	if what == nil {
		panic("Cannot bind nil")
	}
	bindtype := reflect.TypeOf(what)
	if bindtype.Kind() == reflect.Ptr {
		bindtype = bindtype.Elem()
	}
	binding := new(Binding)
	binding.typeof = bindtype
	injector.bindings[bindtype] = append(injector.bindings[bindtype], binding)

	return binding
}

// Override a binding
func (injector *Injector) Override(what interface{}, annotatedWith string) *Binding {
	binding := injector.Bind(what).AnnotatedWith(annotatedWith)
	injector.overrides = append(injector.overrides, &override{typ: binding.typeof, annotatedWith: annotatedWith, binding: binding})

	return binding
}
