#!/usr/bin/env bash

export OCUROOT_HOME=$(pwd)/$(dirname "$0")/testdata/.ocuroot

source $(dirname "$0")/../test_helpers.sh
source $(dirname "$0")/../git_helpers.sh

test_cascade_command() {
    # Clean up test data
    rm -rf ./testdata

    # Call the init function to create repositories and set environment variables
    init_repo "./testdata/remote"

    # Set up working directory with empty initial commit
    init_working_dir "./testdata/source" "$REPO_REMOTE" "SOURCE_WORKING"

    cp ./src/1/* "./testdata/source"

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
    cp ../../src/2/* "./"
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
    init_repo "./testdata/remote"

    # Set up working directory with empty initial commit
    init_working_dir "./testdata/source" "$REPO_REMOTE" "SOURCE_WORKING"

    cp ./src/1/* "./testdata/source"

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
    cp ../../src/2/* "./"
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

setup_test() {

    # Set up environments
    echo "ocuroot release new environments.ocu.star"
    ocuroot release new environments.ocu.star
    assert_equal "0" "$?" "Failed to set up environments"
}

build_ocuroot

pushd "$(dirname "$0")" > /dev/null

test_cascade_command
test_release_cascade

popd > /dev/null
