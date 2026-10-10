package dingo

import (
	"fmt"
	"log/slog"
	"reflect"
	"sync/atomic"
)

// newTrace seeds a circular-dependency trace: empty when tracing is on, nil when off.
func (injector *Injector) newTrace() []circularTraceEntry {
	if injector.circularTracing {
		return make([]circularTraceEntry, 0)
	}

	return nil
}

// resumeTrace gives each provider call its own branch of still-active ancestry.
// A nonnil empty trace keeps tracing enabled even when an ancestor injector owns
// the provider. Providers created without tracing can pick up later enabling.
func (injector *Injector) resumeTrace(captured []circularTraceEntry) []circularTraceEntry {
	if captured == nil {
		return injector.newTrace()
	}

	trace := make([]circularTraceEntry, 0, len(captured))
	for _, entry := range captured {
		if entry.active.Load() {
			trace = append(trace, entry)
		}
	}

	return trace
}

// startConstructionTrace checks live ancestors and records one construction.
// The caller must deactivate the final entry with defer on every exit path.
func startConstructionTrace(trace []circularTraceEntry, typ reflect.Type, annotation string) []circularTraceEntry {
	for _, entry := range trace {
		if entry.typ == typ && entry.annotation == annotation && entry.active.Load() {
			for _, ancestor := range trace {
				if ancestor.active.Load() {
					slog.Info(fmt.Sprintf("%s#%s: %s", ancestor.typ.PkgPath(), ancestor.typ.Name(), ancestor.annotation))
				}
			}

			slog.Info(fmt.Sprintf("%s#%s: %s", typ.PkgPath(), typ.Name(), annotation))

			panic("detected circular dependency")
		}
	}

	active := new(atomic.Bool)
	active.Store(true)

	branch := make([]circularTraceEntry, 0, len(trace)+1)
	branch = append(branch, trace...)

	return append(branch, circularTraceEntry{typ: typ, annotation: annotation, active: active})
}
