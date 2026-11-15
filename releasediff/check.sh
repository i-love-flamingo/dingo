#!/bin/sh
# Compares the observable behaviour of the dingo code in this repository with
# the release required in releasediff/go.mod, and fails unless every
# difference is listed in releasediff/expected.txt. See releasediff/README.md.
#
# Usage: releasediff/check.sh [-update]
#   -update            rewrite expected.txt with the current differences
#   RELEASEDIFF_RACE=1 run the probes against the current code with the race
#                      detector, and fail on any data race
#   RELEASEDIFF_TIMEOUT=<duration> time a probe may run before it counts as a
#                      hang (default 60s)
set -eu

update=false
case "${1:-}" in
	-update) update=true ;;
	"") ;;
	*)
		echo "usage: $0 [-update]" >&2
		exit 2
		;;
esac

dir=$(cd "$(dirname "$0")" && pwd)
expected="$dir/expected.txt"
tmp=$(mktemp -d "${TMPDIR:-/tmp}/releasediff.XXXXXX")
trap 'rm -rf "$tmp"' EXIT
trap 'exit 130' INT TERM

export GOWORK=off LC_ALL=C
race=""
if [ -n "${RELEASEDIFF_RACE:-}" ]; then
	race="-race"
	export CGO_ENABLED=1
fi

release=$(go -C "$dir" list -m -f '{{.Version}}' flamingo.me/dingo)

# probe <name> <what> [go flags]: runs all probes against <what> and writes
# the observations to $tmp/<name>.txt. It fails when go test passes without
# writing any, for example because no test matched.
probe() {
	name=$1 what=$2
	shift 2
	if ! RELEASEDIFF_OUT="$tmp/$name.txt" go -C "$dir" test "$@" -run '^TestReleaseDiff$' -count=1 -timeout 30m . >"$tmp/$name.log" 2>&1; then
		cat "$tmp/$name.log" >&2
		echo "releasediff: running the probes against $what failed" >&2
		exit 2
	fi
	if ! grep -q . "$tmp/$name.txt" 2>/dev/null; then
		cat "$tmp/$name.log" >&2
		echo "releasediff: the probes recorded nothing against $what" >&2
		exit 2
	fi
}

probe release "$release"

# The current code: a copy of go.mod that replaces the release with this
# repository.
cp "$dir/go.mod" "$tmp/current.mod"
cp "$dir/go.sum" "$tmp/current.sum"
go -C "$dir" mod edit -modfile="$tmp/current.mod" -replace=flamingo.me/dingo=../
probe current "the current code" ${race:+"$race"} -mod=mod -modfile="$tmp/current.mod"

{
	comm -23 "$tmp/release.txt" "$tmp/current.txt" | sed 's/^/- /'
	comm -13 "$tmp/release.txt" "$tmp/current.txt" | sed 's/^/+ /'
} | sort -k2 -k1,1r >"$tmp/diff.txt"

if $update; then
	cp "$tmp/diff.txt" "$expected"
	echo "releasediff: wrote $(wc -l <"$tmp/diff.txt" | tr -d ' ') lines to expected.txt"
	exit 0
fi

if ! diff -u "$expected" "$tmp/diff.txt" >"$tmp/mismatch.txt"; then
	cat >&2 <<EOF
releasediff: the behaviour differences between flamingo.me/dingo $release and
this code do not match releasediff/expected.txt.

Each line below starts with two marks. The first compares with expected.txt:
"+" is a new difference, "-" an expected difference that is gone. The second
says where the behaviour was seen: "-" only in $release, "+" only in this code.

EOF
	tail -n +3 "$tmp/mismatch.txt" >&2
	cat >&2 <<EOF

If the change is intended, run releasediff/check.sh -update and explain each
new line of expected.txt in the pull request.
EOF
	exit 1
fi

echo "releasediff: behaviour differences from $release match expected.txt ($(wc -l <"$expected" | tr -d ' ') lines)"
