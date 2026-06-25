package dingo

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/simple"
	"gonum.org/v1/gonum/graph/topo"
)

type (
	// Module is the default entry point for dingo Modules.
	// The Configure method is called once during initialization
	// and lets the module set up Bindings for the provided Injector.
	Module interface {
		Configure(injector *Injector)
	}

	// ModuleFunc wraps a func(injector *Injector) for dependency injection.
	// This allows using small functions as dingo Modules.
	// It follows the same pattern as http.HandlerFunc for http.Handler.
	ModuleFunc func(injector *Injector)

	// Depender returns a list of Modules via the Depends method.
	// This allows a module to specify dependencies, which will be loaded before the actual Module is loaded.
	Depender interface {
		Depends() []Module
	}
)

var (
	ErrModuleCycle = errors.New("cyclic module dependency")
	ErrModuleSort  = errors.New("cannot sort modules")
)

// modGraph is a directed dependency graph of Modules.
//
// Edge direction: an edge A → B means "A must be initialized before B"
// (A is a dependency of B). Cycles are therefore detected as strongly
// connected components with more than one node.
type modGraph struct {
	*simple.DirectedGraph
	idMap map[int64]Module // node ID → Module
	index map[string]int64 // moduleIdentity → node ID
	order map[int64]int    // node ID → registration order (loop index from Add)
}

var typeOfModuleFunc = reflect.TypeOf(ModuleFunc(nil))

// Configure calls the original ModuleFunc with the given *Injector.
func (f ModuleFunc) Configure(injector *Injector) {
	f(injector)
}

// TryModule tests if modules are properly bound
func TryModule(modules ...Module) (resultingError error) {
	defer func() {
		if err := recover(); err != nil {
			if err, ok := err.(error); ok {
				resultingError = err
				return
			}
			resultingError = fmt.Errorf("dingo.TryModule panic: %q", err)
		}
	}()

	injector, err := NewInjector()
	if err != nil {
		return err
	}
	injector.buildEagerSingletons = false
	return injector.InitModules(modules...)
}

// newModuleGraph returns an empty module dependency graph.
func newModuleGraph() *modGraph {
	mg := &modGraph{
		DirectedGraph: simple.NewDirectedGraph(),
		idMap:         make(map[int64]Module),
		index:         make(map[string]int64),
		order:         make(map[int64]int),
	}

	return mg
}

// Add adds each module and its transitive dependencies to the graph.
// A module that is already present (identified by its type or by a pointer
// for ModuleFunc) is skipped — only the first instance is kept.
func (mg *modGraph) Add(modules ...Module) error {
	for i, module := range modules {
		_, err := mg.addModule(i, module)
		if err != nil {
			return err
		}
	}

	return nil
}

// Sort returns all modules in topological order: every dependency appears
// before the modules that depend on it. Ties between independent modules
// are broken by registration order (the loop index from Add), so the result
// preserves the original module insertion order whenever the topological
// constraints allow it.
// An error is returned if the graph contains a cycle.
func (mg *modGraph) Sort() ([]Module, error) {
	sorted, err := topo.SortStabilized(mg, mg.orderByInsertion)
	if err == nil {
		modules := make([]Module, len(sorted))

		for i, node := range sorted {
			modules[i] = mg.idMap[node.ID()]
		}

		return modules, nil
	}

	//TODO: uncomment once go1.27 is released (errors.AsType is available only since go1.26)
	// if cycles, ok := errors.AsType[topo.Unorderable](err); ok && len(cycles) > 0 {

	var cycles topo.Unorderable

	if errors.As(err, &cycles) && len(cycles) > 0 {
		var names []string

		for _, cycle := range cycles {
			for _, node := range cycle {
				if m, found := mg.idMap[node.ID()]; found {
					names = append(names, moduleIdentity(m))
				}
			}

			return nil, fmt.Errorf("%w: %s", ErrModuleCycle, strings.Join(names, " → ")) //nolint:staticcheck // return just the first cycle
		}
	}

	return nil, ErrModuleSort
}

// orderByInsertion is the tiebreaker passed to topo.SortStabilized.
// It sorts a batch of topologically equivalent nodes primarily by their
// registration order (the loop index recorded when each module was first
// added to the graph) so that Sort preserves the original module insertion
// order for modules that are not connected by a dependency edge.
// Node ID (assigned monotonically as nodes are created) is used as a
// secondary key so that ties within an equal-order group — typically
// transitive dependencies pulled in by the same top-level module — are
// resolved in the order they were created, yielding a deterministic result.
func (mg *modGraph) orderByInsertion(nodes []graph.Node) {
	slices.SortStableFunc(nodes, func(a, b graph.Node) int {
		if d := mg.order[a.ID()] - mg.order[b.ID()]; d != 0 {
			return d
		}

		return int(a.ID() - b.ID())
	})
}

// addModule inserts a single module into the graph (if not already present)
// and recursively inserts all modules returned by its Depends method.
// The order parameter is the registration index of the top-level module that
// pulled this module in (its position in the slice passed to Add); transitive
// dependencies inherit the parent's order so that they cluster together with
// their dependent in the tiebreaker. Modules that are already present keep
// the order from their first registration.
// It returns the graph node ID assigned to the module.
func (mg *modGraph) addModule(order int, module Module) (int64, error) {
	key := moduleIdentity(module)

	processed, ok := mg.index[key]
	if ok {
		return processed, nil
	}

	newNode := mg.NewNode()
	mg.index[key] = newNode.ID()
	mg.idMap[newNode.ID()] = module
	mg.order[newNode.ID()] = order
	mg.AddNode(newNode)

	if depender, ok := module.(Depender); ok {
		for _, dep := range depender.Depends() {
			depID, err := mg.addModule(order, dep)
			if err != nil {
				return 0, fmt.Errorf("could not add module: %w", err)
			}

			depNode := mg.Node(depID)
			isDependencyOf := mg.NewEdge(depNode, newNode) // depNode is a dependency of newNode

			mg.SetEdge(isDependencyOf)
		}
	}

	return newNode.ID(), nil
}

// moduleIdentity returns a stable string key that uniquely identifies a module.
// For ordinary module types the key is the fully qualified type name.
// For ModuleFunc values the function pointer address is included so that
// two distinct func literals are treated as different modules.
func moduleIdentity(module Module) string {
	modType := reflect.TypeOf(module)
	if modType == typeOfModuleFunc {
		value := reflect.ValueOf(module)
		return fmt.Sprintf("%s_%d", value.Type(), value.Pointer())
	}

	return modType.String()
}
