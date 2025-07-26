#!/bin/bash

export OCU_REPO_COMMIT_OVERRIDE=${OCU_REPO_COMMIT_OVERRIDE:-commitid}

source $(dirname "$0")/../test_helpers.sh

test_print_secret() {
    echo "Test: secret printing"
    echo ""
    setup_test

    echo "== release v1 =="
    RELEASE_OUTPUT=$(ocuroot release new release.ocu.star)
    assert_equal "0" "$?" "Failed to release v1"

    COUNT=$(echo "$RELEASE_OUTPUT" | grep "abc123" | wc -l | xargs)
    N=$'\n'
    assert_equal "0" "$COUNT" "Release output contains secret $COUNT times. Output was:$N$RELEASE_OUTPUT"
    
    echo "Test succeeded"
    echo ""
}

setup_test() {
    # Clean up any previous runs
    rm -rf .store
}

build_ocuroot

pushd "$(dirname "$0")" > /dev/null

test_print_secret

popd > /dev/null
