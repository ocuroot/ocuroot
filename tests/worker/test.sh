#!/usr/bin/env bash

export OCUROOT_HOME=$(pwd)/$(dirname "$0")/testdata/.ocuroot
export TESTDATA_DIR=$(pwd)/$(dirname "$0")/testdata
export OCUROOT_DEBUG=true

source $(dirname "$0")/../test_helpers.sh
source $(dirname "$0")/../git_helpers.sh

test_worker_push() {
     # Clean up test data
    rm -rf ./testdata

    # Call the init function to create repositories and set environment variables
    export REPO_REMOTE=$(init_repo "./testdata/remote")

    # Set up working directory with empty initial commit
    init_working_dir "./testdata/source" "$REPO_REMOTE" "SOURCE_WORKING"

    cp ./src/repo1/commit1/* "./testdata/source"

    pushd "./testdata/source" >> /dev/null

    # Start worker in the source directory, with dev mode
    echo "Starting worker, will log to: ../worker.log"
    ocuroot start worker --dev --interval 1s > ../worker.log 2>&1 &
    assert_equal "0" "$?" "Failed to start worker"
    sleep 3 # Allow the worker to come up

    # Apply first commit
    git add .
    git commit -m "Add source"

    # Wait until deploys are complete
    wait_for_ref "push/-/a.ocu.star/@/deploy/production"
    wait_for_ref "push/-/b.ocu.star/@/deploy/production"

    assert_deployed "a.ocu.star" "production"
    assert_deployed "b.ocu.star" "production"
    assert_ref_equals "push/-/a.ocu.star/@/deploy/production#output/message" "Message at commit 1"
    assert_ref_equals "push/-/b.ocu.star/@/deploy/production#output/message" "Message at commit 1"

    # Apply second commit
    cp ../../src/repo1/commit2/* "./"

    git add .
    git commit -m "Update message"

    # Wait until deploys are complete
    wait_for_ref "push/-/a.ocu.star/@r2/deploy/production"

    assert_deployed "a.ocu.star" "production"
    assert_deployed "b.ocu.star" "production"
    assert_ref_equals "push/-/a.ocu.star/@/deploy/production#output/message" "Message at commit 2"
    assert_ref_equals "push/-/b.ocu.star/@/deploy/production#output/message" "Message at commit 2"

    check_ref_does_not_exist "push/-/b.ocu.star/@r2/deploy/production"

    popd >> /dev/null

    echo "Test succeeded"
    echo ""
}


test_multi_worker_push() {
     # Clean up test data
    rm -rf ./testdata

    # Call the init function to create repositories and set environment variables
    export REPO_REMOTE=$(init_repo "./testdata/remote")

    # Set up working directory with empty initial commit
    init_working_dir "./testdata/source" "$REPO_REMOTE" "SOURCE_WORKING"

    cp ./src/repo1/commit1/* "./testdata/source"

    pushd "./testdata/source" >> /dev/null

    # Start worker in the source directory, with dev mode
    echo "Starting 3 workers, will log to: ../worker{1,2,3}.log"
    OCUROOT_HOME=$TESTDATA_DIR/worker1 ocuroot start worker --dev --interval 1s > ../worker1.log 2>&1 &
    worker1_pid=$!
    OCUROOT_HOME=$TESTDATA_DIR/worker2 ocuroot start worker --dev --interval 1s > ../worker2.log 2>&1 &
    worker2_pid=$!
    OCUROOT_HOME=$TESTDATA_DIR/worker3 ocuroot start worker --dev --interval 1s > ../worker3.log 2>&1 &
    worker3_pid=$!
    sleep 3 # Allow the workers to come up

    # Apply first commit
    git add .
    git commit -m "Add source"

    # Wait until deploys are complete
    wait_for_ref "push/-/a.ocu.star/@/deploy/production"
    wait_for_ref "push/-/b.ocu.star/@/deploy/production"

    assert_deployed "a.ocu.star" "production"
    assert_deployed "b.ocu.star" "production"
    assert_ref_equals "push/-/a.ocu.star/@/deploy/production#output/message" "Message at commit 1"
    assert_ref_equals "push/-/b.ocu.star/@/deploy/production#output/message" "Message at commit 1"

    # Apply second commit
    cp ../../src/repo1/commit2/* "./"

    git add .
    git commit -m "Update message"

    # Wait until deploys are complete
    wait_for_ref "push/-/a.ocu.star/@r2/deploy/production"

    assert_deployed "a.ocu.star" "production"
    assert_deployed "b.ocu.star" "production"
    assert_ref_equals "push/-/a.ocu.star/@/deploy/production#output/message" "Message at commit 2"
    assert_ref_equals "push/-/b.ocu.star/@/deploy/production#output/message" "Message at commit 2"

    # Ensure b did not get another full release
    check_ref_does_not_exist "push/-/b.ocu.star/@r2/deploy/production"
    # Ensure the parallel workers did not cause extra work
    check_ref_does_not_exist "push/-/b.ocu.star/@r1/deploy/production/3"

    popd >> /dev/null

    # Check the worker pids are still running
    if ! kill -0 $worker1_pid 2>/dev/null; then
        echo "Worker 1 is not running"
        exit 1
    fi
    if ! kill -0 $worker2_pid 2>/dev/null; then
        echo "Worker 2 is not running"
        exit 1
    fi
    if ! kill -0 $worker3_pid 2>/dev/null; then
        echo "Worker 3 is not running"
        exit 1
    fi


    echo "Test succeeded"
    echo ""
}

test_worker_intent() {
    # Clean up test data
    rm -rf ./testdata

    # Call the init function to create repositories and set environment variables
    export REPO_REMOTE=$(init_repo "./testdata/remote")

    # Set up working directory with empty initial commit
    init_working_dir "./testdata/source" "$REPO_REMOTE" "SOURCE_WORKING"

    cp ./src/repo1/commit1/* "./testdata/source"

    pushd "./testdata/source" >> /dev/null

    # Start worker in the source directory, with dev mode
    echo "Starting worker, will log to: ../worker.log"
    ocuroot start worker --dev --interval 1s > ../worker.log 2>&1 &
    assert_equal "0" "$?" "Failed to start worker"
    sleep 3 # Allow the worker to come up

    # Apply first commit
    git add .
    git commit -m "Add source"

    # Wait until deploys are complete
    wait_for_ref "push/-/a.ocu.star/@/deploy/production"
    wait_for_ref "push/-/b.ocu.star/@/deploy/production"

    assert_deployed "a.ocu.star" "production"
    assert_deployed "b.ocu.star" "production"
    assert_ref_equals "push/-/a.ocu.star/@/deploy/production#output/message" "Message at commit 1"
    assert_ref_equals "push/-/b.ocu.star/@/deploy/production#output/message" "Message at commit 1"

    # Set intent
    ocuroot state set "@/custom/test" 1
    assert_equal "0" "$?" "Failed to set intent"

    # Wait until intent updates
    wait_for_ref "@/custom/test"
    assert_ref_equals "@/custom/test" "1"

    popd >> /dev/null

    echo "Test succeeded"
    echo ""
}

build_ocuroot

pushd "$(dirname "$0")" > /dev/null

# test_worker_push
test_multi_worker_push
# test_worker_intent

popd