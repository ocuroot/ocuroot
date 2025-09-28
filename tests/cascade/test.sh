#!/usr/bin/env bash

export OCUROOT_HOME=$(pwd)/$(dirname "$0")/testdata/.ocuroot
export OCUROOT_DEBUG=true

source $(dirname "$0")/../test_helpers.sh
source $(dirname "$0")/../git_helpers.sh

test_cascade_command() {
    # Clean up test data
    rm -rf ./testdata

    # Call the init function to create repositories and set environment variables
    export REPO_REMOTE=$(init_repo "./testdata/remote")

    # Set up working directory with empty initial commit
    init_working_dir "./testdata/source" "$REPO_REMOTE" "SOURCE_WORKING"

    cp ./src/repo1/1/* "./testdata/source"

    pushd "./testdata/source" >> /dev/null

    git add .
    git commit -m "Add source"
    git remote -v
    git push

    setup_test

    ocuroot release new a.ocu.star
    assert_equal "0" "$?" "Failed to deploy a"
    ocuroot release new b.ocu.star
    assert_equal "0" "$?" "Failed to deploy b"

    assert_deployed "a.ocu.star" "production"
    assert_deployed "b.ocu.star" "production"
    assert_ref_equals "cascade/-/a.ocu.star/@/deploy/production#output/message" "Message at commit 1"
    assert_ref_equals "cascade/-/b.ocu.star/@/deploy/production#output/message" "Message at commit 1"

    echo "== commit change 2 =="
    cp ../../src/repo1/2/* "./"
    git add .
    git commit -m "Add change 2"
    git push

    ocuroot release new a.ocu.star
    assert_equal "0" "$?" "Failed to deploy a"

    assert_ref_equals "cascade/-/a.ocu.star/@/deploy/production#output/message" "Message at commit 2"
    assert_ref_equals "cascade/-/b.ocu.star/@/deploy/production#output/message" "Message at commit 1"

    echo "== cascade work 2 =="
    ocuroot work cascade
    assert_equal "0" "$?" "Failed to cascade work"

    assert_ref_equals "cascade/-/a.ocu.star/@/deploy/production#output/message" "Message at commit 2"
    assert_ref_equals "cascade/-/b.ocu.star/@/deploy/production#output/message" "Message at commit 2"

    popd >> /dev/null

    echo "Test succeeded"
    echo ""
}

test_release_cascade() {
    # Clean up test data
    rm -rf ./testdata

    # Call the init function to create repositories and set environment variables
    export REPO_REMOTE=$(init_repo "./testdata/remote")

    # Set up working directory with empty initial commit
    init_working_dir "./testdata/source" "$REPO_REMOTE" "SOURCE_WORKING"

    cp ./src/repo1/1/* "./testdata/source"

    pushd "./testdata/source" >> /dev/null
    
    git add .
    git commit -m "Add source"
    git remote -v
    git push

    setup_test

    ocuroot release new a.ocu.star
    assert_equal "0" "$?" "Failed to deploy a"
    ocuroot release new b.ocu.star
    assert_equal "0" "$?" "Failed to deploy b"

    assert_deployed "a.ocu.star" "production"
    assert_deployed "b.ocu.star" "production"
    assert_ref_equals "cascade/-/a.ocu.star/@/deploy/production#output/message" "Message at commit 1"
    assert_ref_equals "cascade/-/b.ocu.star/@/deploy/production#output/message" "Message at commit 1"

    echo "== commit change 2 =="
    cp ../../src/repo1/2/* "./"
    git add .
    git commit -m "Add change 2"
    git push

    ocuroot state set "@/custom/test" "foo"
    check_ref_does_not_exist "@/custom/test"
    
    ocuroot release new a.ocu.star --cascade
    assert_equal "0" "$?" "Failed to deploy a"

    assert_ref_equals "cascade/-/a.ocu.star/@/deploy/production#output/message" "Message at commit 2"
    assert_ref_equals "cascade/-/b.ocu.star/@/deploy/production#output/message" "Message at commit 2"
    check_ref_does_not_exist "@/custom/test"
    
    echo "Final check to apply custom state"
    ocuroot work cascade
    assert_equal "0" "$?" "Failed to apply custom state"
    assert_ref_equals "@/custom/test" "foo"

    popd >> /dev/null

    echo "Test succeeded"
    echo ""
}

test_release_cascade_cross_repo() {
    # Clean up test data
    rm -rf ./testdata

    # Call the init function to create repositories and set environment variables
    export REPO_REMOTE=$(init_repo "./testdata/remote")
    export REPO2_REMOTE=$(init_repo "./testdata/remote2")

    # Set up working directory with empty initial commit
    init_working_dir "./testdata/source" "$REPO_REMOTE" "SOURCE_WORKING"
    init_working_dir "./testdata/store" "$REPO_REMOTE" "STORE_WORKING"
    init_working_dir "./testdata/source2" "$REPO2_REMOTE" "SOURCE_WORKING2"

    pushd "./testdata/store" >> /dev/null
    
    # Create state and intent branches
    # This avoids them sharing code
    git checkout -b state
    git push origin --set-upstream state
    git checkout -b intent
    git push origin --set-upstream intent

    popd >> /dev/null

    cp ./src/repo1/1/* "./testdata/source"
    cp ./src/repo2/1/* "./testdata/source2"

    pushd "./testdata/source" >> /dev/null
    git checkout main

    git add .
    git commit -m "Add source"
    git remote -v
    git push

    setup_test

    ocuroot release new a.ocu.star
    assert_equal "0" "$?" "Failed to deploy a"

    assert_deployed "a.ocu.star" "production"
    assert_ref_equals "cascade/-/a.ocu.star/@/deploy/production#output/message" "Message at commit 1"

    popd >> /dev/null

    pushd "./testdata/source2" >> /dev/null

    git add .
    git commit -m "Add source"
    git remote -v
    git push

    ocuroot release new b.ocu.star
    assert_equal "0" "$?" "Failed to deploy b"

    assert_deployed "b.ocu.star" "production"
    assert_ref_equals "cascade_repo2/-/b.ocu.star/@/deploy/production#output/message" "Message at commit 1"

    popd >> /dev/null

    pushd "./testdata/source" >> /dev/null
    
    echo "== commit change 2 =="
    cp ../../src/repo1/2/* "./"
    git add .
    git commit -m "Add change 2"
    git push

    ocuroot state set "@/custom/test" "foo"
    check_ref_does_not_exist "@/custom/test"
    
    ocuroot release new a.ocu.star --cascade
    assert_equal "0" "$?" "Failed to deploy a"

    assert_ref_equals "cascade/-/a.ocu.star/@/deploy/production#output/message" "Message at commit 2"
    assert_ref_equals "cascade_repo2/-/b.ocu.star/@/deploy/production#output/message" "Message at commit 2"
    check_ref_does_not_exist "@/custom/test"

    popd >> /dev/null

    pushd "./testdata/store" >> /dev/null

    git pull
    git fetch origin intent
    git checkout intent
    assert_equal "0" "$?" "Failed to checkout intent"

    echo "Apply custom state from intent repo"
    ocuroot work cascade
    assert_equal "0" "$?" "Failed to apply custom state"

    # Switch to main to check current state
    # TODO: This shouldn't be necessary
    git checkout main
    git pull

    assert_ref_equals "@/custom/test" "foo"

    popd >> /dev/null

    echo "Test succeeded"
    echo ""
}

setup_test() {

    # Set up environments
    echo "ocuroot release new environments.ocu.star"
    ocuroot release new environments.ocu.star
    assert_equal "0" "$?" "Failed to set up environments"
}

build_ocuroot

pushd "$(dirname "$0")" > /dev/null

# test_cascade_command
# test_release_cascade
test_release_cascade_cross_repo

popd > /dev/null
