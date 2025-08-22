#!/bin/bash

export ENABLE_OTEL=true
export OCUROOT_CHECK_STAGED_FILES=true
export OCUROOT_DEBUG_TRACES=true

# Avoid confusion between main and master in some git configs
export DEFAULT_BRANCH_NAME=dbranch

source $(dirname "$0")/../test_helpers.sh
source $(dirname "$0")/test_helpers.sh

test_ocuroot_release() {
    echo "Test: Ocuroot Release via CI"
    echo ""

    rm -rf testdata
    mkdir -p testdata
    TESTDATA=$(realpath testdata)

    # Start the CI server in background
    start_ci_server
    trap cleanup_ci_server RETURN
    
    TEST_REPO_DIR=$(create_repo "testdata")
    echo "Test repository created at $TEST_REPO_DIR"

    JOB_ID=$(schedule_job "$TEST_REPO_DIR/repo.git" "$DEFAULT_BRANCH_NAME" "./release.sh $TESTDATA/ocuroot_home/1")
    assert_equal "0" "$?" "Failed to schedule job"

    wait_for_all_jobs "$TESTDATA/logs"

    assert_equal 1 $(job_count) "Expected one job, found $(job_count)"

    JOB_STATUS=$(job_status "$JOB_ID")
    assert_equal "success" "$JOB_STATUS" "Job $JOB_ID did not succeed, status: $JOB_STATUS"
    job_logs "$JOB_ID"

    REPO_DIR=$(checkout_repo "$TEST_REPO_DIR/repo.git" "testdata/working")
    assert_equal "0" "$?" "Failed to checkout repository"

    pushd "$REPO_DIR" > /dev/null
    assert_deployed "release.ocu.star" "staging"
    assert_deployed "release.ocu.star" "production"
    assert_deployed "release.ocu.star" "production2"
    popd > /dev/null

    echo "Test completed"
    echo ""
}

test_ocuroot_release_deps() {
    echo "Test: Ocuroot Release (with dependencies) via CI"
    echo ""

    rm -rf testdata
    mkdir -p testdata
    TESTDATA=$(realpath testdata)

    # Start the CI server in background
    start_ci_server
    trap cleanup_ci_server RETURN
    
    TEST_REPO_DIR=$(create_repo "testdata")
    echo "Test repository created at $TEST_REPO_DIR"

    JOB_ID=$(schedule_job "$TEST_REPO_DIR/repo.git" "$DEFAULT_BRANCH_NAME" "./release-deps.sh $TESTDATA/ocuroot_home/1")
    assert_equal "0" "$?" "Failed to schedule job"

    wait_for_all_jobs "$TESTDATA/logs"

    assert_equal 2 $(job_count) "Expected two jobs, found $(job_count)"
    assert_job_success

    REPO_DIR=$(checkout_repo "$TEST_REPO_DIR/repo.git" "testdata/working")
    assert_equal "0" "$?" "Failed to checkout repository"

    pushd "$REPO_DIR" > /dev/null
    assert_deployed "frontend.ocu.star" "staging"
    assert_deployed "frontend.ocu.star" "production"
    assert_deployed "frontend.ocu.star" "production2"
    assert_deployed "backend.ocu.star" "staging"
    assert_deployed "backend.ocu.star" "production"
    assert_deployed "backend.ocu.star" "production2"

    assert_ref_equals "./-/backend.ocu.star/@/deploy/staging#output/message" "1"
    assert_ref_equals "./-/frontend.ocu.star/@/deploy/staging#output/message" "1"

    popd > /dev/null

    echo "Test completed"
    echo ""
}

test_ocuroot_release_deps_commits() {
    echo "Test: Ocuroot Release (with dependencies) via CI, multiple commits"
    echo ""

    rm -rf testdata
    mkdir -p testdata
    TESTDATA=$(realpath testdata)

    # Start the CI server in background
    start_ci_server
    trap cleanup_ci_server RETURN
    
    TEST_REPO_DIR=$(create_repo "testdata")
    echo "Test repository created at $TEST_REPO_DIR"

    # Only need to check out once to get state consistently
    REPO_DIR=$(checkout_repo "$TEST_REPO_DIR/repo.git" "testdata/working")
    assert_equal "0" "$?" "Failed to checkout repository"

    echo "Repository checked out to $REPO_DIR"

    j=1
    for i in $(seq 1 3); do
        echo "Test iteration $i"

        checkout_and_modify_repo "$TEST_REPO_DIR/repo.git" "testdata/iteration-i$i" "message-backend.txt" "$i"
        if [ $((i % 2)) -eq 0 ]; then
            j=$((i / 2))
            checkout_and_modify_repo "$TEST_REPO_DIR/repo.git" "testdata/iteration-j$j" "message-frontend.txt" "$j"
        fi

        JOB_ID=$(schedule_job "$TEST_REPO_DIR/repo.git" $DEFAULT_BRANCH_NAME "./release-deps.sh")
        assert_equal "0" "$?" "Failed to schedule job"

        wait_for_all_jobs "$TESTDATA/logs"

        pushd "$REPO_DIR" > /dev/null
        echo "i = $i, j = $j"
        assert_ref_equals "./-/backend.ocu.star/@/deploy/staging#output/message" "$i"
        assert_ref_equals "./-/frontend.ocu.star/@/deploy/staging#output/message" "$j"
        assert_ref_equals "./-/frontend.ocu.star/@/deploy/staging#output/backend_message" "$i"
        popd > /dev/null
    done

    echo "Job count: $(job_count)"

    echo "Test completed"
    echo ""
}

test_intent_change() {
    echo "Test: Intent change via CI"
    echo ""

    rm -rf testdata
    mkdir -p testdata
    TESTDATA=$(realpath testdata)

    # Start the CI server in background
    start_ci_server
    trap cleanup_ci_server RETURN

    TEST_REPO_DIR=$(create_repo "testdata")
    echo "Test repository created at $TEST_REPO_DIR"

    # Check out repo to make state available
    REPO_DIR=$(checkout_repo "$TEST_REPO_DIR/repo.git" "testdata/working")
    pushd "$REPO_DIR" > /dev/null

    JOB_ID=$(schedule_job "$TEST_REPO_DIR/repo.git" "$DEFAULT_BRANCH_NAME" "./release.sh $TESTDATA/ocuroot_home/1")
    assert_equal "0" "$?" "Failed to schedule job"

    wait_for_all_jobs "$TESTDATA/logs"

    assert_deployed "release.ocu.star" "staging"
    assert_ref_equals "./-/release.ocu.star/@/deploy/staging#output/foo" "bar"

    ocuroot state set "+/custom/foo" "baz"
    assert_equal "0" "$?" "Failed to update state"

    JOB_ID=$(schedule_job "$TEST_REPO_DIR/repo.git" "$DEFAULT_BRANCH_NAME" "./intent.sh $TESTDATA/ocuroot_home/2")
    assert_equal "0" "$?" "Failed to schedule intent update"

    wait_for_all_jobs "$TESTDATA/logs"
    assert_job_success

    assert_ref_equals "@/custom/foo" "baz"
    assert_deployed "release.ocu.star" "staging"
    assert_ref_equals "./-/release.ocu.star/@/deploy/staging#output/foo" "baz"

    popd > /dev/null

    echo "Test completed"
    echo ""
}

# Get rid of any previous runs
cleanup_dangling_ci

# Make sure the binary is up to date
build_ocuroot

# Run tests
pushd "$(dirname "$0")" > /dev/null

test_ocuroot_release
test_ocuroot_release_deps
test_ocuroot_release_deps_commits
test_intent_change

popd > /dev/null

echo "All tests completed"
