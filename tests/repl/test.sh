#!/bin/bash

export OCUROOT_HOME=$(pwd)/$(dirname "$0")/testdata/.ocuroot
export OCUROOT_DEBUG=true

source $(dirname "$0")/../test_helpers.sh

rm -rf ./testdata

build_ocuroot

pushd $(dirname "$0") > /dev/null

ocuroot repl test.ocu.star -c "fn(\"hello\")"
assert_equal "0" "$?" "Failed to run repl"

ocuroot repl test.ocu.star -c "print(GLOBAL_VAR)"
assert_equal "0" "$?" "Failed to run repl"

popd > /dev/null