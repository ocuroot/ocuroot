#!/bin/bash

source $(dirname "$0")/../test_helpers.sh

bad_inputs() {
    setup_test

    ocuroot release new badinputs.ocu.star
    assert_not_equal "0" "$?" "Expected a failure"

    ocuroot release new badinputsinnext.ocu.star
    assert_not_equal "0" "$?" "Expected a failure"

    echo "Test passed"
}

setup_test() {
    rm -rf .store
}

build_ocuroot

pushd "$(dirname "$0")" > /dev/null
bad_inputs
popd > /dev/null
