#!/bin/bash

export OCU_REPO_COMMIT_OVERRIDE=${OCU_REPO_COMMIT_OVERRIDE:-commitid}

source $(dirname "$0")/../test_helpers.sh

test_two_releases() {
    echo "Test: releases"
    echo ""
    setup_test

    echo "== release v1 =="
    ocuroot release new approvals.ocu.star
    assert_equal "0" "$?" "Failed to release v1"

    echo "== create approval intent =="
    ocuroot state set "minimal/repo/-/approvals.ocu.star/+v1/custom/approval" 1
    assert_equal "0" "$?" "Failed to create approval intent"
    echo "== apply approval intent =="
    ocuroot state apply "minimal/repo/-/approvals.ocu.star/+v1/custom/approval"
    assert_equal "0" "$?" "Failed to apply approval intent"
    echo "== continue release up to second approval =="
    ocuroot release continue minimal/repo/-/approvals.ocu.star/@v1
    assert_equal "0" "$?" "Failed to continue release up to second approval"
    echo "== create approval2 intent =="
    ocuroot state set "minimal/repo/-/approvals.ocu.star/+v1/custom/approval2" 1
    assert_equal "0" "$?" "Failed to create approval2 intent"
    echo "== apply approval2 intent =="
    ocuroot state apply "minimal/repo/-/approvals.ocu.star/+v1/custom/approval2"
    assert_equal "0" "$?" "Failed to apply approval2 intent"

    echo "== trigger work from state root =="
    pushd ".store/state" > /dev/null
    ocuroot work trigger
    assert_equal "0" "$?" "Failed to trigger work"
    popd > /dev/null


    echo "== continue work =="
    ocuroot work continue
    assert_equal "0" "$?" "Failed to continue work"

    assert_deployed "approvals.ocu.star" "staging"
    assert_deployed "approvals.ocu.star" "production"
    assert_deployed "approvals.ocu.star" "production2"

    echo "== continue work - should be none =="
    ocuroot work continue
    assert_equal "0" "$?" "Should be no work to continue"

    echo ""
    echo "v2"
    echo ""

    echo "== release v2 =="
    ocuroot release new approvals.ocu.star --force
    assert_equal "0" "$?" "Failed to release v2"
    echo "== create approval intent =="
    ocuroot state set "minimal/repo/-/approvals.ocu.star/+v2/custom/approval" 1
    assert_equal "0" "$?" "Failed to create approval intent"
    echo "== apply approval intent =="
    ocuroot state apply "minimal/repo/-/approvals.ocu.star/+v2/custom/approval"
    assert_equal "0" "$?" "Failed to apply approval intent"
    echo "== continue release up to second approval =="
    ocuroot release continue minimal/repo/-/approvals.ocu.star/@v2
    assert_equal "0" "$?" "Failed to continue release up to second approval"
    echo "== create approval2 intent =="
    ocuroot state set "minimal/repo/-/approvals.ocu.star/+v2/custom/approval2" 1
    assert_equal "0" "$?" "Failed to create approval2 intent"
    echo "== apply approval2 intent =="
    ocuroot state apply "minimal/repo/-/approvals.ocu.star/+v2/custom/approval2"
    assert_equal "0" "$?" "Failed to apply approval2 intent"
    echo "== continue release =="
    ocuroot release continue minimal/repo/-/approvals.ocu.star/@v2
    assert_equal "0" "$?" "Failed to continue release"

    echo "Test succeeded"
    echo ""
}

test_down() {
    echo "Test: down"
    echo ""
    setup_test

    echo "== release v1 =="
    ocuroot release new basic.ocu.star
    assert_equal "0" "$?" "Failed to release v1"

    assert_deployed "basic.ocu.star" "staging"
    assert_deployed "basic.ocu.star" "production"
    assert_deployed "basic.ocu.star" "production2"

    echo "== down v1 =="
    ocuroot deploy down minimal/repo/-/basic.ocu.star/@/deploy/production
    assert_equal "0" "$?" "Failed to down v1"

    assert_deployed "basic.ocu.star" "staging"
    assert_not_deployed "basic.ocu.star" "production"
    assert_deployed "basic.ocu.star" "production2"

    echo "Test succeeded"
    echo ""
}

test_deploy_intent() {
    echo "Test: deploy intent"
    echo ""
    setup_test

    echo "== release v1 =="
    ocuroot release new basic.ocu.star
    assert_equal "0" "$?" "Failed to release v1"

    assert_deployed "basic.ocu.star" "staging"
    assert_deployed "basic.ocu.star" "production"
    assert_deployed "basic.ocu.star" "production2"

    echo "== delete deployment intent =="
    ocuroot state delete minimal/repo/-/basic.ocu.star/+/deploy/production
    assert_equal "0" "$?" "Failed to delete deployment intent"

    echo "== apply all outstanding intents =="
    ocuroot state diff | xargs -n1 ocuroot state apply
    assert_equal "0" "$?" "Failed to diff state"

    echo "== continue =="
    ocuroot work continue
    assert_equal "0" "$?" "Failed to continue work"

    assert_deployed "basic.ocu.star" "staging"
    assert_not_deployed "basic.ocu.star" "production"
    assert_deployed "basic.ocu.star" "production2"

    echo "Test succeeded"
    echo ""
}

setup_test() {
    # Clean up any previous runs
    rm -rf .store

    # Set up environments
    echo "ocuroot release new environments/package.ocu.star"
    ocuroot release new environments/package.ocu.star
    assert_equal "0" "$?" "Failed to set up environments"
}

build_ocuroot

pushd "$(dirname "$0")" > /dev/null

test_two_releases
test_down
test_deploy_intent

popd > /dev/null
