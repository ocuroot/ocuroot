#!/usr/bin/env bash

export OCUROOT_HOME=$(pwd)/$(dirname "$0")/.ocuroot

source $(dirname "$0")/../test_helpers.sh

bad_inputs() {
    setup_test

    ocuroot release new badinputs.ocu.star
    assert_not_equal "0" "$?" "Expected a failure in badinputs.ocu.star"

    ocuroot release new badinputsinnext.ocu.star
    assert_not_equal "0" "$?" "Expected a failure in badinputsinnext.ocu.star"

    ocuroot release new tags.ocu.star
    assert_not_equal "0" "$?" "Expected a failure in tags.ocu.star"

    ocuroot release new tags2.ocu.star
    assert_not_equal "0" "$?" "Expected a failure in tags2.ocu.star"

    ocuroot release new environments.ocu.star
    assert_not_equal "0" "$?" "Expected a failure in environments.ocu.star"

    check_ref_does_not_exist "@/environment/invalid/name"

    ocuroot state set -f=json "+/environment/bad+environment" '{"attributes": {"type": "prod"},"name": "bad+environment"}'
    ocuroot work any

    check_ref_does_not_exist "@/environment/bad+environment"
    ocuroot state delete "+/environment/bad+environment"

    ocuroot state set -f=json "+/environment/shouldmatch" '{"attributes": {"type": "prod"},"name": "non_matching"}'
    ocuroot work any

    check_ref_does_not_exist "@/environment/shouldmatch"

    ocuroot release new functionargs.ocu.star
    assert_not_equal "0" "$?" "Expected a failure in functionargs.ocu.star"
    
    echo "Test passed"
}

setup_test() {
    rm -rf .store
    rm -rf .ocuroot
}

build_ocuroot

pushd "$(dirname "$0")" > /dev/null
bad_inputs
popd > /dev/null
