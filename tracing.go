package dingo

import (
	"fmt"
	"log/slog"
	"reflect"
	"sync/atomic"
)

// providerReentryLimit is how many live repeats of one construction a trace
// branch may reach through generated provider calls before tracing reports a
// cycle. Repeats with no provider call in between are reported at once.
const providerReentryLimit = 16

// newTrace seeds a circular-dependency trace: empty when tracing is on, nil when off.
func (injector *Injector) newTrace() []circularTraceEntry {
	if injector.circularTracing {
		return make([]circularTraceEntry, 0)
	}

	return nil
}

// resumeTrace gives each provider call its own branch of still-active ancestry.
// Copied entries are marked viaProvider, so repeats of them count toward
// providerReentryLimit instead of being reported at once.
// A nonnil empty trace keeps tracing enabled even when an ancestor injector owns
// the provider. Providers created without tracing can pick up later enabling.
func (injector *Injector) resumeTrace(captured []circularTraceEntry) []circularTraceEntry {
	if captured == nil {
		return injector.newTrace()
	}

	trace := make([]circularTraceEntry, 0, len(captured))
	for _, entry := range captured {
		if entry.active.Load() {
			entry.viaProvider = true
			trace = append(trace, entry)
		}
	}

	return trace
}

// startConstructionTrace checks live ancestors and records one construction.
// A repeat with no provider call in between is reported at once. Repeats through
// provider calls are counted in this branch only, so overlapping calls from
// other goroutines do not add up.
// The caller must end the trace with defer finishTrace on every exit path.
func startConstructionTrace(trace []circularTraceEntry, typ reflect.Type, annotation string) []circularTraceEntry {
	newest, repeats := -1, 0

	for i := len(trace) - 1; i >= 0; i-- {
		entry := trace[i]
		if entry.scope != nil || entry.typ != typ || entry.annotation != annotation || !entry.active.Load() {
			continue
		}

		if newest < 0 {
			newest = i
		}

		repeats++
		if !entry.viaProvider || repeats > providerReentryLimit {
			reportCircular(trace[newest:], typ, annotation)
		}
	}

	return appendTrace(trace, circularTraceEntry{typ: typ, annotation: annotation})
}

// startScopeTrace records a resolution in a locking scope before the scope locks.
// A repeat in the same scope with no provider call in between would wait for
// itself forever, so it is reported at once. A repeat through a provider call
// may come from another goroutine that rightly waits, so it is only logged.
// The caller must end the trace with defer finishTrace on every exit path.
func startScopeTrace(trace []circularTraceEntry, scope Scope, typ reflect.Type, annotation string) []circularTraceEntry {
	for i := len(trace) - 1; i >= 0; i-- {
		entry := trace[i]
		if entry.scope != scope || entry.typ != typ || entry.annotation != annotation || !entry.active.Load() {
			continue
		}

		if !entry.viaProvider {
			reportCircular(trace[i:], typ, annotation)
		}

		slog.Warn(fmt.Sprintf("%s#%s: %s is still being resolved, waiting; a hang here is a circular dependency", typ.PkgPath(), typ.Name(), annotation))

		break
	}

	return appendTrace(trace, circularTraceEntry{typ: typ, annotation: annotation, scope: scope})
}

// appendTrace copies the branch and adds a new active entry.
func appendTrace(trace []circularTraceEntry, entry circularTraceEntry) []circularTraceEntry {
	entry.active = new(atomic.Bool)
	entry.active.Store(true)

	branch := make([]circularTraceEntry, 0, len(trace)+1)
	branch = append(branch, trace...)

	return append(branch, entry)
}

// finishTrace ends the newest entry of a trace.
func finishTrace(trace []circularTraceEntry) {
	trace[len(trace)-1].active.Store(false)
}

// reportCircular logs one lap of the cycle, from its newest repeat, and panics.
func reportCircular(lap []circularTraceEntry, typ reflect.Type, annotation string) {
	for _, ancestor := range lap {
		if ancestor.active.Load() {
			slog.Info(fmt.Sprintf("%s#%s: %s", ancestor.typ.PkgPath(), ancestor.typ.Name(), ancestor.annotation))
		}
	}

	slog.Info(fmt.Sprintf("%s#%s: %s", typ.PkgPath(), typ.Name(), annotation))

	panic("detected circular dependency")
}
