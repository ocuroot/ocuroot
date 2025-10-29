#!/usr/bin/env bash

export OCU_REPO_COMMIT_OVERRIDE=${OCU_REPO_COMMIT_OVERRIDE:-commitid}
export OCUROOT_HOME=$(pwd)/$(dirname "$0")/testdata/.ocuroot

source $(dirname "$0")/../test_helpers.sh
source $(dirname "$0")/../git_helpers.sh

test_gitstate() {
    # Call the init function to create repositories and set environment variables
    init_repos

    # Set up working directories with initial commits
    init_working_dir "$(pwd)/testdata/state" "$STATE_REMOTE" "STATE_WORKING"
    init_working_dir "$(pwd)/testdata/intent" "$INTENT_REMOTE" "INTENT_WORKING"

    setup_test

    ocuroot release new basic.ocu.star
    assert_equal "0" "$?" "Failed to deploy basic package"

    assert_deployed "basic.ocu.star" "staging"
    assert_deployed "basic.ocu.star" "production"
    assert_deployed "basic.ocu.star" "production2"

    echo "== delete deployment intent =="
    ocuroot state delete ./-/basic.ocu.star/@/deploy/production
    assert_equal "0" "$?" "Failed to delete deployment intent"

    echo "== check out intent store =="
    INTENT_CHECKOUT="$(pwd)/testdata/intent_checkout"
    git clone "$INTENT_REMOTE" "$INTENT_CHECKOUT"
    assert_equal "0" "$?" "Failed to clone intent store"

    echo "== trigger update from intent store =="
    pushd "$INTENT_CHECKOUT" > /dev/null

    git checkout intent
    assert_equal "0" "$?" "Failed to checkout intent branch"

    # Trigger an intent change
    ocuroot work trigger --intent
    assert_equal "0" "$?" "Failed to trigger update"
    assert_file_exists ".triggers/commitid"
    popd > /dev/null

    echo "== apply intents =="
    ocuroot state diff | xargs -n1 ocuroot state apply
    assert_equal "0" "$?" "Failed to apply intents"

    echo "== continue work =="
    ocuroot work continue
    assert_equal "0" "$?" "Failed to continue work"

    assert_deployed "basic.ocu.star" "staging"
    assert_not_deployed "basic.ocu.star" "production"
    assert_deployed "basic.ocu.star" "production2"

    echo "== check for files in state store =="
    check_file_in_remote "$STATE_REMOTE" "state" "support.txt" "state"

    echo "== check author in state store =="
    check_last_log_in_remote "$STATE_REMOTE" "state" "Author: Ocuroot <contact@ocuroot.com>"

    echo "== check for files in intent store =="
    check_file_in_remote "$INTENT_REMOTE" "intent" "support.txt" "intent"

    echo "Test succeeded"
    echo ""
}

setup_test() {
    echo "State remote: $STATE_REMOTE"
    echo "Intent remote: $INTENT_REMOTE"

    rm -rf testdata

    # Set up environments
    echo "ocuroot release new environments.ocu.star"
    ocuroot release new environments.ocu.star
    assert_equal "0" "$?" "Failed to set up environments"
}

build_ocuroot

pushd "$(dirname "$0")" > /dev/null

test_gitstate

popd > /dev/null
