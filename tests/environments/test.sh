#!/usr/bin/env bash

export OCUROOT_HOME=$(pwd)/$(dirname "$0")/.ocuroot
export OCUROOT_DEBUG=true
export OCU_REPO_COMMIT_OVERRIDE=${OCU_REPO_COMMIT_OVERRIDE:-commitid}

source $(dirname "$0")/../test_helpers.sh

create_environment() {

    setup_test

    ocuroot release new environments.ocu.star
    assert_equal "0" "$?" "Failed to release environments"

    ocuroot release new package1.ocu.star
    assert_equal "0" "$?" "Failed to release package1"

    ocuroot release new package2.ocu.star
    assert_equal "0" "$?" "Failed to release package2"

    check_ref_exists "package1.ocu.star/@/deploy/production2"
    check_ref_exists "package2.ocu.star/@/deploy/production2"

    ocuroot state set -f=json "@/environment/production3" '{"attributes": {"type": "prod"},"name": "production3"}'
    assert_equal "0" "$?" "Failed to set environment"
    ocuroot state apply "@/environment/production3"
    assert_equal "0" "$?" "Failed to apply environment"

    check_ref_exists "package1.ocu.star/@r1/op/check_envs"

    ocuroot work ops
    assert_equal "0" "$?" "Failed to run ops"

    # The task must have been removed once fulfilled
    check_ref_does_not_exist "package1.ocu.star/@r1/op/check_envs"

    ocuroot work continue
    assert_equal "0" "$?" "Failed to continue work"

    check_ref_exists "package1.ocu.star/@/deploy/production3"

    echo "Test passed"
    echo ""
}

create_environment_omnibus() {
    export OCU_REPO_COMMIT_OVERRIDE=${OCU_REPO_COMMIT_OVERRIDE:-commitid}

    setup_test

    ocuroot release new environments.ocu.star
    assert_equal "0" "$?" "Failed to release environments"

    ocuroot release new package1.ocu.star
    assert_equal "0" "$?" "Failed to release package1"

    ocuroot release new package2.ocu.star
    assert_equal "0" "$?" "Failed to release package2"

    check_ref_exists "package1.ocu.star/@/deploy/production2"
    check_ref_exists "package2.ocu.star/@/deploy/production2"

    ocuroot state set -f=json "@/environment/production3" '{"attributes": {"type": "prod"},"name": "production3"}'
    assert_equal "0" "$?" "Failed to set environment"

    ocuroot work any
    assert_equal "0" "$?" "Failed to sync environment"

    # The task must have been removed once fulfilled
    check_ref_exists "package1.ocu.star/@/deploy/production3"
    check_ref_does_not_exist "package1.ocu.star/@1/op/check_envs"

    echo "Test passed"
    echo ""
}


delete_environment() {
    echo "delete_environment"
    echo "------------------"

    setup_test

    ocuroot release new environments.ocu.star
    assert_equal "0" "$?" "Failed to release environments"

    ocuroot release new package1.ocu.star
    assert_equal "0" "$?" "Failed to release package1"

    ocuroot release new package2.ocu.star
    assert_equal "0" "$?" "Failed to release package2"

    check_ref_exists "package1.ocu.star/@/deploy/production2"
    check_ref_exists "package2.ocu.star/@/deploy/production2"

    check_file_exists "./.deploys/production2/package1.txt"
    check_file_exists "./.deploys/production2/package2.txt"

    ocuroot state delete "@/environment/production2"
    ocuroot state apply "@/environment/production2"

    check_ref_does_not_exist "@/environment/production2"    

    ocuroot work continue

    check_ref_does_not_exist "package1.ocu.star/@/deploy/production2"

    check_file_exists "./.deploys/staging/package1.txt"
    check_file_exists "./.deploys/staging/package2.txt"
    check_file_exists "./.deploys/production/package1.txt"
    check_file_exists "./.deploys/production/package2.txt"

    check_file_does_not_exist "./.deploys/production2/package1.txt"
    check_file_does_not_exist "./.deploys/production2/package2.txt"

    echo "Test passed"
    echo ""
}

delete_environment_omnibus() {
    echo "delete_environment_omnibus"
    echo "--------------------------"

    setup_test

    echo "=== Releasing environments ==="
    ocuroot release new environments.ocu.star
    assert_equal "0" "$?" "Failed to release environments"

    echo "=== Releasing package1 ==="
    ocuroot release new package1.ocu.star
    assert_equal "0" "$?" "Failed to release package1"

    echo "=== Releasing package2 ==="
    ocuroot release new package2.ocu.star
    assert_equal "0" "$?" "Failed to release package2"

    check_ref_exists "package1.ocu.star/@/deploy/production2"
    check_ref_exists "package2.ocu.star/@/deploy/production2"

    check_file_exists "./.deploys/production2/package1.txt"
    check_file_exists "./.deploys/production2/package2.txt"

    echo "=== Deleting environment ==="
    ocuroot state delete "@/environment/production2"

    echo "=== Running work any ==="
    ocuroot work any

    check_ref_does_not_exist "@/environment/production2"    

    check_ref_does_not_exist "package1.ocu.star/@/deploy/production2"

    check_file_exists "./.deploys/staging/package1.txt"
    check_file_exists "./.deploys/staging/package2.txt"
    check_file_exists "./.deploys/production/package1.txt"
    check_file_exists "./.deploys/production/package2.txt"

    check_file_does_not_exist "./.deploys/production2/package1.txt"
    check_file_does_not_exist "./.deploys/production2/package2.txt"

    echo "=== Repeating work any ==="

    # This shouldn't change anything
    ocuroot work any

    check_ref_does_not_exist "@/environment/production2"    

    check_ref_does_not_exist "package1.ocu.star/@/deploy/production2"

    check_file_exists "./.deploys/staging/package1.txt"
    check_file_exists "./.deploys/staging/package2.txt"
    check_file_exists "./.deploys/production/package1.txt"
    check_file_exists "./.deploys/production/package2.txt"

    check_file_does_not_exist "./.deploys/production2/package1.txt"
    check_file_does_not_exist "./.deploys/production2/package2.txt"

    echo "Test passed"
    echo ""
}

delete_environment_comprehensive() {
    echo "delete_environment_comprehensive"
    echo "--------------------------------"

    setup_test

    echo "=== Releasing environments ==="
    ocuroot release new environments.ocu.star
    assert_equal "0" "$?" "Failed to release environments"

    echo "=== Releasing package1 ==="
    ocuroot release new package1.ocu.star
    assert_equal "0" "$?" "Failed to release package1"

    echo "=== Releasing package2 ==="
    ocuroot release new package2.ocu.star
    assert_equal "0" "$?" "Failed to release package2"

    check_ref_exists "package1.ocu.star/@/deploy/production2"
    check_ref_exists "package2.ocu.star/@/deploy/production2"

    check_file_exists "./.deploys/production2/package1.txt"
    check_file_exists "./.deploys/production2/package2.txt"

    echo "=== Deleting environment ==="
    ocuroot state delete "@/environment/production2"

    echo "=== Running work any ==="
    ocuroot work any --comprehensive

    check_ref_does_not_exist "@/environment/production2"    

    check_ref_does_not_exist "package1.ocu.star/@/deploy/production2"

    check_file_exists "./.deploys/staging/package1.txt"
    check_file_exists "./.deploys/staging/package2.txt"
    check_file_exists "./.deploys/production/package1.txt"
    check_file_exists "./.deploys/production/package2.txt"

    check_file_does_not_exist "./.deploys/production2/package1.txt"
    check_file_does_not_exist "./.deploys/production2/package2.txt"

    echo "=== Repeating work any ==="

    # This shouldn't change anything
    ocuroot work any --comprehensive

    check_ref_does_not_exist "@/environment/production2"    

    check_ref_does_not_exist "package1.ocu.star/@/deploy/production2"

    check_file_exists "./.deploys/staging/package1.txt"
    check_file_exists "./.deploys/staging/package2.txt"
    check_file_exists "./.deploys/production/package1.txt"
    check_file_exists "./.deploys/production/package2.txt"

    check_file_does_not_exist "./.deploys/production2/package1.txt"
    check_file_does_not_exist "./.deploys/production2/package2.txt"

    echo "Test passed"
    echo ""
}

setup_test() {
    rm -rf .store
    rm -rf .ocuroot
    rm -rf .deploys
}

build_ocuroot

pushd "$(dirname "$0")" > /dev/null
create_environment
create_environment_omnibus
delete_environment
delete_environment_omnibus
delete_environment_comprehensive
popd > /dev/null
