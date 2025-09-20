#!/usr/bin/env bash

export ENABLE_OTEL=true
export OCUROOT_DEBUG=true
export OCUROOT_DEBUG_TRACES=true
export OCUROOT_CHECK_STAGED_FILES=true

export OCU_REPO_COMMIT_OVERRIDE=${OCU_REPO_COMMIT_OVERRIDE:-commitid}

export OCUROOT_HOME=$(pwd)/$(dirname "$0")/.ocuroot

source $(dirname "$0")/../test_helpers.sh

test_explicit_version() {
    echo "Test: explicit version 0.3.14"
    echo ""
    setup_test

    echo "== release with explicit version 0.3.14 =="
    ocuroot release new version_test.ocu.star
    assert_equal "0" "$?" "Failed to release with explicit version"

    # Verify the build task completed
    check_ref_exists "version_test.ocu.star/@r1/task/build/1/status/complete"
    
    # Verify the deploy task completed
    check_ref_exists "version_test.ocu.star/@r1/task/deploy_test/1/status/complete"

    echo "Test succeeded"
    echo ""
}

test_no_version() {
    echo "Test: no explicit version (automatic resolution)"
    echo ""
    setup_test

    echo "== release without explicit version call =="
    ocuroot release new no_version_test.ocu.star
    assert_equal "0" "$?" "Failed to release without explicit version"

    # Verify the build task completed
    check_ref_exists "no_version_test.ocu.star/@r1/task/build/1/status/complete"
    
    # Verify the deploy task completed
    check_ref_exists "no_version_test.ocu.star/@r1/task/deploy_test/1/status/complete"

    echo "Test succeeded"
    echo ""
}

setup_test() {
    # Clean up any previous runs
    rm -rf .store
    rm -rf $OCUROOT_HOME
}

build_ocuroot

test_explicit_version
test_no_version

echo "All SDK version tests passed!"