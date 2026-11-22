#!/bin/sh
# Usage: downstream.sh <head dir> <base dir> <consumer dir> [go test flags]
# Runs the tests of the consumer module with the dingo in <head dir>. When they fail,
# runs them again with the dingo in <base dir>. Exit code 1 only if the tests pass
# with the base and fail with the head. Exit code 2 if the consumer dir has no go.mod.
# The script resets and changes go.mod and go.sum in the consumer dir, so use a clean clone.

export GOWORK=off

head=$(cd "$1" && pwd)
base=$(cd "$2" && pwd)
consumer=$3
shift 3

if [ ! -f "$consumer/go.mod" ]; then
	echo "::error::$consumer has no go.mod"
	exit 2
fi

# run <dingo dir> [go test flags]
# Joined with &&, because set -e has no effect in an if condition.
run() {
	dingo=$1
	shift
	(
		cd "$consumer" &&
			git checkout -- go.mod go.sum &&
			go mod edit -replace "flamingo.me/dingo=$dingo" &&
			go mod tidy &&
			go test "$@" ./...
	)
}

if run "$head" "$@"; then
	exit 0
fi

echo "downstream: the tests of $consumer fail with this change, running them again with the base commit"

if run "$base" "$@"; then
	echo "::error::the tests of $consumer pass with the base commit and fail with this change"
	exit 1
fi

echo "::warning::the tests of $consumer fail also with the base commit, so this change does not block"
