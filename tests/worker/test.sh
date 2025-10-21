#!/usr/bin/env bash

export OCUROOT_HOME=$(pwd)/$(dirname "$0")/testdata/.ocuroot
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

test_worker_push
test_worker_intent

popd