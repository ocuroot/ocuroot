#!/usr/bin/env bash

export OCUROOT_HOME=$(pwd)/$(dirname "$0")/testdata/.ocuroot
export OCUROOT_DEBUG=true

source $(dirname "$0")/../test_helpers.sh
source $(dirname "$0")/../git_helpers.sh

test_push_from_source() {
     # Clean up test data
    rm -rf ./testdata

    # Call the init function to create repositories and set environment variables
    export REPO_REMOTE=$(init_repo "./testdata/remote")

    # Set up working directory with empty initial commit
    init_working_dir "./testdata/source" "$REPO_REMOTE" "SOURCE_WORKING"

    cp ./src/repo1/commit1/* "./testdata/source"

    pushd "./testdata/source" >> /dev/null

    # Apply first commit
    git add .
    git commit -m "Add source"
    git remote -v
    git push

    ocuroot push
    assert_equal "0" "$?" "Failed to push"

    assert_deployed "a.ocu.star" "production"
    assert_deployed "b.ocu.star" "production"
    assert_ref_equals "push/-/a.ocu.star/@/deploy/production#output/message" "Message at commit 1"
    assert_ref_equals "push/-/b.ocu.star/@/deploy/production#output/message" "Message at commit 1"

    # Apply second commit
    cp ../../src/repo1/commit2/* "./"

    # Uncommitted files should fail the push
    ocuroot push
    assert_not_equal "0" "$?" "Should have failed to push"

    git add .

    # Staged changes should fail the push
    ocuroot push
    assert_not_equal "0" "$?" "Should have failed to push"

    git commit -m "Update message"
    git remote -v
    git push

    ocuroot push
    assert_equal "0" "$?" "Failed to push"

    assert_deployed "a.ocu.star" "production"
    assert_deployed "b.ocu.star" "production"
    assert_ref_equals "push/-/a.ocu.star/@/deploy/production#output/message" "Message at commit 2"
    assert_ref_equals "push/-/b.ocu.star/@/deploy/production#output/message" "Message at commit 2"

    check_ref_exists "push/-/repo.ocu.star/@/push/index"

    check_ref_does_not_exist "push/-/b.ocu.star/@r2/deploy/production"

    popd >> /dev/null

    echo "Test succeeded"
    echo ""
}


test_push_from_intent() {
     # Clean up test data
    rm -rf ./testdata

    # Call the init function to create repositories and set environment variables
    export REPO_REMOTE=$(init_repo "./testdata/remote")

    # Set up working directory with empty initial commit
    init_working_dir "./testdata/source" "$REPO_REMOTE" "SOURCE_WORKING"

    cp ./src/repo1/commit1/* "./testdata/source"

    pushd "./testdata/source" >> /dev/null

    # Apply first commit
    git add .
    git commit -m "Add source"
    git remote -v
    git push

    ocuroot push
    assert_equal "0" "$?" "Failed to push"

    assert_deployed "a.ocu.star" "production"
    assert_deployed "b.ocu.star" "production"
    assert_ref_equals "push/-/a.ocu.star/@/deploy/production#output/message" "Message at commit 1"
    assert_ref_equals "push/-/b.ocu.star/@/deploy/production#output/message" "Message at commit 1"

    ocuroot state set "@/custom/test" 1
    assert_equal "0" "$?" "Failed to set state"

    git fetch
    git checkout intent

    ocuroot push
    assert_equal "0" "$?" "Failed to push"

    git checkout main

    assert_ref_equals "@/custom/test" "1"
    ocuroot state set "@/custom/test2" 1
    assert_equal "0" "$?" "Failed to set state"
    
    git fetch
    git checkout intent
    
    ocuroot push
    assert_equal "0" "$?" "Failed to push"
    
    git checkout main
    
    assert_ref_equals "@/custom/test" "1"
    assert_ref_equals "@/custom/test2" "1"
    
    popd >> /dev/null

    echo "Test succeeded"
    echo ""
}

build_ocuroot

pushd "$(dirname "$0")" > /dev/null

test_push_from_source
test_push_from_intent

popd