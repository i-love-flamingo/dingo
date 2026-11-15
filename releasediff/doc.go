// Package releasediff compares the observable behaviour of the dingo code in
// this repository with the release required in go.mod. The probes live in the
// test files; check.sh runs them against both and compares the differences
// with expected.txt.
package releasediff
