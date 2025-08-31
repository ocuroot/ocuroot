#!/usr/bin/env bash

source $(dirname "$0")/../test_helpers.sh

retry_explicit_ref() {
    setup_test

    ocuroot release new release.ocu.star
    assert_not_equal "0" "$?" "Expected a failure"

    check_ref_does_not_exist "release.ocu.star/@/deploy/production"

    ocuroot release retry release.ocu.star/@1
    assert_equal "0" "$?" "Expected retry to succeed"

    check_ref_exists "release.ocu.star/@/deploy/production"
    # Ensure that the release was completed after the retry
    check_ref_exists "release.ocu.star/@1/call/postrelease/1/status/complete"

    echo "Test passed"
}

retry_package_only() {
    setup_test

    ocuroot release new release.ocu.star
    assert_not_equal "0" "$?" "Expected a failure"

    check_ref_does_not_exist "release.ocu.star/@/deploy/production"

    ocuroot release retry release.ocu.star
    assert_equal "0" "$?" "Expected retry to succeed"

    check_ref_exists "release.ocu.star/@/deploy/production"

    ocuroot release retry release.ocu.star
    assert_not_equal "0" "$?" "Expected second retry to fail"

    echo "Test passed"
}

setup_test() {
    rm -rf .store
    rm -rf .data

    ocuroot release new environments.ocu.star
}

build_ocuroot

pushd "$(dirname "$0")" > /dev/null
retry_explicit_ref
retry_package_only
popd > /dev/null
