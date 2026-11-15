package releasediff_test

// The harness runs every probe in its own subprocess, so a panic, stack
// overflow or deadlock in dingo ends only that probe. Each probe runs in two
// modes: "plain", and "trace" with dingo.EnableCircularTracing called before
// any injector exists. A probe records observations as label=value pairs.
// TestReleaseDiff writes one sorted line per observation to the file named by
// RELEASEDIFF_OUT:
//
//	<probe> <mode> <label>=<value>

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"reflect"
	"regexp"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"flamingo.me/dingo"
)

const (
	envOut     = "RELEASEDIFF_OUT"
	envProbe   = "RELEASEDIFF_PROBE"
	envMode    = "RELEASEDIFF_MODE"
	envTimeout = "RELEASEDIFF_TIMEOUT"

	obsPrefix     = "obs "
	harnessPrefix = "harness-error "
	doneMarker    = "probe-done"

	// maxStack keeps stack overflows fast; the default limit is 1 GB.
	maxStack       = 64 << 20
	defaultTimeout = 60 * time.Second
	maxLogLines    = 20
)

var modes = []string{"plain", "trace"}

// probes maps a probe name to its function. Each probe file registers its
// probes in init.
var probes = map[string]func(*recorder){}

func register(name string, probe func(*recorder)) {
	if _, ok := probes[name]; ok {
		panic("releasediff: duplicate probe " + name)
	}

	probes[name] = probe
}

// recorder writes each observation at once, so the ones made before a crash
// are kept.
type recorder struct {
	seen map[string]bool
}

// labelPattern keeps labels free of spaces and "=", so lines stay easy to read.
var labelPattern = regexp.MustCompile(`^[^\s=]+$`)

func (r *recorder) add(label, value string) {
	if !labelPattern.MatchString(label) || r.seen[label] {
		fmt.Fprintf(os.Stdout, "%sinvalid or repeated label %q\n", harnessPrefix, label)
		os.Exit(3)
	}

	r.seen[label] = true
	fmt.Fprintf(os.Stdout, "%s%s=%s\n", obsPrefix, label, clean(value))
}

// logOutput collects what dingo logs through slog during a probe.
var logOutput syncBuffer

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.Write(p)
}

// lines returns the logged lines from the message on, without time and level.
func (b *syncBuffer) lines() []string {
	b.mu.Lock()
	defer b.mu.Unlock()

	var lines []string

	for line := range strings.SplitSeq(b.buf.String(), "\n") {
		if line == "" {
			continue
		}

		if i := strings.Index(line, "msg="); i >= 0 {
			line = line[i:]
		}

		lines = append(lines, line)
	}

	return lines
}

// countLogLines counts the logged lines that contain substr.
func countLogLines(substr string) int {
	n := 0

	for _, line := range logOutput.lines() {
		if strings.Contains(line, substr) {
			n++
		}
	}

	return n
}

var addressPattern = regexp.MustCompile(`0x[0-9a-f]+`)

// clean makes a value deterministic and keeps it on one line.
func clean(s string) string {
	s = addressPattern.ReplaceAllString(s, "0xADDR")

	return strings.ReplaceAll(s, "\n", " / ")
}

// show prints a value with its type, using Name() where the type has one.
func show(v any) string {
	if v == nil {
		return "nil"
	}

	if rv := reflect.ValueOf(v); rv.Kind() == reflect.Pointer && rv.IsNil() {
		return fmt.Sprintf("%T(nil)", v)
	}

	if named, ok := v.(interface{ Name() string }); ok {
		return fmt.Sprintf("%T{%s}", v, catch(named.Name))
	}

	return fmt.Sprintf("%T{%v}", v, v)
}

// result prints the outcome of a call returning a value and an error.
func result(v any, err error) string {
	if err != nil {
		return "ERR(" + err.Error() + ")"
	}

	return show(v)
}

// errText prints the outcome of a call returning only an error.
func errText(err error) string {
	if err != nil {
		return "ERR(" + err.Error() + ")"
	}

	return "ok"
}

// catch runs f and prints a panic instead of its result.
func catch(f func() string) (s string) {
	defer func() {
		if p := recover(); p != nil {
			s = fmt.Sprintf("PANIC(%v)", p)
		}
	}()

	return f()
}

func catchErr(f func() error) string {
	return catch(func() string { return errText(f()) })
}

func identity(a, b any) string {
	if a == b {
		return "same"
	}

	return "distinct"
}

// TestProbe is the subprocess body: it runs the probe named by
// RELEASEDIFF_PROBE and prints its observations.
func TestProbe(t *testing.T) {
	t.Parallel()

	name := os.Getenv(envProbe)
	if name == "" {
		t.Skip("runs only as a subprocess of TestReleaseDiff")
	}

	probe, ok := probes[name]
	if !ok {
		t.Fatalf("unknown probe %q", name)
	}

	debug.SetMaxStack(maxStack)
	slog.SetDefault(slog.New(slog.NewTextHandler(&logOutput, &slog.HandlerOptions{Level: slog.LevelDebug})))

	if os.Getenv(envMode) == "trace" {
		dingo.EnableCircularTracing()
	}

	rec := &recorder{seen: map[string]bool{}}

	func() {
		defer func() {
			if p := recover(); p != nil {
				rec.add("panic", fmt.Sprint(p))
			}
		}()

		probe(rec)
	}()

	logged := logOutput.lines()
	if len(logged) > 0 {
		rec.add("log.lines", strconv.Itoa(len(logged)))
	}

	for i, line := range logged[:min(len(logged), maxLogLines)] {
		rec.add(fmt.Sprintf("log.%02d", i+1), line)
	}

	fmt.Fprintln(os.Stdout, doneMarker)
}

// TestReleaseDiff runs every probe in every mode and writes the sorted
// observations to RELEASEDIFF_OUT.
func TestReleaseDiff(t *testing.T) {
	t.Parallel()

	out := os.Getenv(envOut)
	if out == "" {
		t.Skip("set " + envOut + "; check.sh runs this")
	}

	timeout := defaultTimeout

	if s := os.Getenv(envTimeout); s != "" {
		var err error
		if timeout, err = time.ParseDuration(s); err != nil {
			t.Fatalf("%s: %v", envTimeout, err)
		}
	}

	var (
		mu    sync.Mutex
		lines []string
		wg    sync.WaitGroup
		slots = make(chan struct{}, runtime.NumCPU())
	)

	for name := range probes {
		for _, mode := range modes {
			wg.Go(func() {
				slots <- struct{}{}
				defer func() { <-slots }()

				observed, err := runProbe(name, mode, timeout)
				if err != nil {
					t.Errorf("%s %s: %v", name, mode, err)
				}

				mu.Lock()
				defer mu.Unlock()

				for _, obs := range observed {
					lines = append(lines, name+" "+mode+" "+obs)
				}
			})
		}
	}

	wg.Wait()
	sort.Strings(lines)

	//nolint:gosec // the path comes from check.sh
	if err := os.WriteFile(out, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

// runProbe runs one probe in a subprocess and returns its observations. When
// the subprocess ends before the probe finished, the last observation says
// how it ended. A data race, or an end the output does not explain, is an
// error.
func runProbe(name, mode string, timeout time.Duration) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	//nolint:gosec // runs this test binary again, with fixed arguments
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestProbe$", "-test.count=1")

	cmd.Env = append(os.Environ(), envProbe+"="+name, envMode+"="+mode, envOut+"=")

	var stdout, stderr bytes.Buffer

	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	runErr := cmd.Run()

	var (
		observed []string
		done     bool
	)

	for line := range strings.SplitSeq(stdout.String(), "\n") {
		switch {
		case strings.HasPrefix(line, harnessPrefix):
			return nil, errors.New(strings.TrimPrefix(line, harnessPrefix))
		case strings.HasPrefix(line, obsPrefix):
			observed = append(observed, strings.TrimPrefix(line, obsPrefix))
		case line == doneMarker:
			done = true
		}
	}

	if strings.Contains(stderr.String(), "WARNING: DATA RACE") {
		return nil, errors.New("data race:\n" + stderr.String())
	}

	if !done {
		reason, ok := exitReason(ctx, stdout.String()+stderr.String())
		if !ok {
			if runErr == nil {
				runErr = errors.New("exit status 0")
			}

			return nil, fmt.Errorf("ended without a result: %w\n%s%s", runErr, stdout.String(), stderr.String())
		}

		observed = append(observed, "exit="+reason)
	}

	return observed, nil
}

// exitReason says how a probe subprocess ended early, or false when its
// output does not tell. A deadlock and a timeout are both "hang", because
// whether Go reports a deadlock depends on the platform, the Go version and
// the race detector.
func exitReason(ctx context.Context, output string) (string, bool) {
	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded),
		strings.Contains(output, "all goroutines are asleep"):
		return "hang", true
	case strings.Contains(output, "goroutine stack exceeds"):
		return "stack overflow", true
	}

	for line := range strings.SplitSeq(output, "\n") {
		if strings.HasPrefix(line, "panic:") || strings.HasPrefix(line, "fatal error:") {
			return "crash: " + clean(line), true
		}
	}

	return "", false
}
