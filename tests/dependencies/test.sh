#!/usr/bin/env bash

export OCU_REPO_COMMIT_OVERRIDE=${OCU_REPO_COMMIT_OVERRIDE:-commitid}

source $(dirname "$0")/../test_helpers.sh

test_releases_with_continue() {
    echo "Test: releases with continue"
    echo ""
    setup_test
    
    # Release the frontend from repo root
    echo "ocuroot release new ./-/frontend/package.ocu.star"
    ocuroot release new ./-/frontend/package.ocu.star
    assert_equal "0" "$?" "Failed to release frontend"

    # Should not have deployed due to dependency on backend
    assert_not_deployed "frontend/package.ocu.star" "staging"
    assert_not_deployed "frontend/package.ocu.star" "production"
    assert_not_deployed "frontend/package.ocu.star" "production2"

    # Release the backend, from backend dir
    pushd backend
    echo "ocuroot release new package.ocu.star"
    ocuroot release new package.ocu.star
    assert_equal "0" "$?" "Failed to release backend"
    popd

    # Should have deployed, no dependencies
    assert_deployed "backend/package.ocu.star" "staging"
    assert_deployed "backend/package.ocu.star" "production"
    assert_deployed "backend/package.ocu.star" "production2"

    # The frontend needs to be updated again
    ocuroot work continue
    assert_equal "0" "$?" "Failed to continue work on this commit"

    assert_deployed "frontend/package.ocu.star" "staging"
    assert_deployed "frontend/package.ocu.star" "production"
    assert_deployed "frontend/package.ocu.star" "production2"

    assert_ref_equals "./-/backend/package.ocu.star/@/deploy/staging#output/credential" "abcd"
    assert_ref_equals "./-/frontend/package.ocu.star/@/deploy/staging#output/backend_credential" "abcd"

    echo "Test succeeded"
    echo ""
}

test_releases_with_intent_update() {
    echo "Test: releases with intent update"
    echo ""
    setup_test

    ocuroot release new backend/package.ocu.star
    assert_equal "0" "$?" "Failed to release backend"

    ocuroot release new ./-/frontend/package.ocu.star
    assert_equal "0" "$?" "Failed to release frontend"

    echo "Setting and applying backend credential intent"
    ocuroot state set "./-/backend/package.ocu.star/+/custom/credential/staging" "efgh"
    assert_equal "0" "$?" "Failed to update backend credential"
    ocuroot state apply "./-/backend/package.ocu.star/+/custom/credential/staging"
    assert_equal "0" "$?" "Failed to apply backend credential"

    # TODO: In ocuroot work trigger, identify stale deployments and run continue.

    echo "Continuing work to capture backend credential"
    ocuroot work continue
    assert_equal "0" "$?" "Failed to continue work on this commit"

    echo "Continuing work to capture frontend update"
    ocuroot work continue
    assert_equal "0" "$?" "Failed to continue work on this commit"

    assert_ref_equals "./-/backend/package.ocu.star/@/deploy/staging#output/credential" "efgh"
    assert_ref_equals "./-/frontend/package.ocu.star/@/deploy/staging#output/backend_credential" "efgh"

    echo "Test succeeded"
    echo ""
}

test_releases_across_commits() {
    echo "Test: releases across commits"
    echo ""
    setup_test

    OCU_REPO_COMMIT_OVERRIDE=commit2 ocuroot release new ./-/frontend/package.ocu.star
    assert_equal "0" "$?" "Failed to release frontend"

    OCU_REPO_COMMIT_OVERRIDE=commit1 ocuroot release new backend/package.ocu.star
    assert_equal "0" "$?" "Failed to release backend"

    echo "Triggering work to capture commits"
    ocuroot work trigger
    assert_equal "0" "$?" "Failed to trigger work"
    assert_file_exists "./.store/triggers/commit2"
    rm "./.store/triggers/commit2"

    echo "Continuing work on wrong commit should do nothing"
    ocuroot work continue
    assert_equal "0" "$?" "Failed to continue work on this commit"

    assert_deployed "backend/package.ocu.star" "staging"
    assert_not_deployed "frontend/package.ocu.star" "staging"

    echo "Continuing work to finish frontend deployments"
    OCU_REPO_COMMIT_OVERRIDE=commit2 ocuroot work continue
    assert_equal "0" "$?" "Failed to continue work on this commit"

    assert_ref_equals "./-/backend/package.ocu.star/@/deploy/staging#output/credential" "abcd"
    assert_ref_equals "./-/frontend/package.ocu.star/@/deploy/staging#output/backend_credential" "abcd"

    echo "Setting and applying backend credential intent"
    ocuroot state set "./-/backend/package.ocu.star/+/custom/credential/staging" "efgh"
    assert_equal "0" "$?" "Failed to update backend credential"
    ocuroot state apply "./-/backend/package.ocu.star/+/custom/credential/staging"
    assert_equal "0" "$?" "Failed to apply backend credential"

    echo "Triggering work to capture credential for backend"
    ocuroot work trigger
    assert_equal "0" "$?" "Failed to trigger work"
    assert_file_exists "./.store/triggers/commit1"
    rm "./.store/triggers/commit1"

    echo "Continuing work to capture backend credential"
    OCU_REPO_COMMIT_OVERRIDE=commit1 ocuroot work continue
    assert_equal "0" "$?" "Failed to continue work on this commit"

    # Should have only triggered backend
    assert_ref_equals "./-/backend/package.ocu.star/@/deploy/staging#output/credential" "efgh"
    assert_ref_equals "./-/frontend/package.ocu.star/@/deploy/staging#output/backend_credential" "abcd"

    echo "Triggering work to capture backend update for frontend"
    ocuroot work trigger
    assert_equal "0" "$?" "Failed to trigger work"
    assert_file_exists "./.store/triggers/commit2"
    rm "./.store/triggers/commit2"

    echo "Continuing work to capture frontend update"
    OCU_REPO_COMMIT_OVERRIDE=commit2 ocuroot work continue
    assert_equal "0" "$?" "Failed to continue work on this commit"

    # Should be complete
    assert_ref_equals "./-/backend/package.ocu.star/@/deploy/staging#output/credential" "efgh"
    assert_ref_equals "./-/frontend/package.ocu.star/@/deploy/staging#output/backend_credential" "efgh"

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

test_releases_with_continue
test_releases_with_intent_update
test_releases_across_commits

popd > /dev/null