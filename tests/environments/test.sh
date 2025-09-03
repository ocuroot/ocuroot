#!/usr/bin/env bash

export OCUROOT_HOME=$(pwd)/$(dirname "$0")/.ocuroot

source $(dirname "$0")/../test_helpers.sh

create_environment() {
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

    ocuroot state set -f=json "+/environment/production3" '{"attributes": {"type": "prod"},"name": "production3"}'
    assert_equal "0" "$?" "Failed to set environment"
    ocuroot state apply "+/environment/production3"
    assert_equal "0" "$?" "Failed to apply environment"

    check_ref_exists "package1.ocu.star/@r1/task/check_envs"

    ocuroot work tasks
    assert_equal "0" "$?" "Failed to run tasks"

    # The task must have been removed once fulfilled
    check_ref_does_not_exist "package1.ocu.star/@r1/task/check_envs"

    ocuroot work continue
    assert_equal "0" "$?" "Failed to continue work"

    check_ref_exists "package1.ocu.star/@/deploy/production3"

    echo "Test passed"
}

delete_environment() {
    setup_test

    ocuroot release new environments.ocu.star
    assert_equal "0" "$?" "Failed to release environments"

    ocuroot release new package1.ocu.star
    assert_equal "0" "$?" "Failed to release package1"

    ocuroot release new package2.ocu.star
    assert_equal "0" "$?" "Failed to release package2"

    check_ref_exists "package1.ocu.star/@/deploy/production2"
    check_ref_exists "package2.ocu.star/@/deploy/production2"

    ocuroot state delete "+/environment/production2"
    ocuroot state apply "+/environment/production2"

    check_ref_does_not_exist "@/environment/production2"    

    ocuroot work continue

    check_ref_does_not_exist "package1.ocu.star/@/deploy/production2"
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

    ocuroot state set -f=json "+/environment/production3" '{"attributes": {"type": "prod"},"name": "production3"}'
    assert_equal "0" "$?" "Failed to set environment"

    ocuroot work any
    assert_equal "0" "$?" "Failed to sync environment"

    # The task must have been removed once fulfilled
    check_ref_exists "package1.ocu.star/@/deploy/production3"
    check_ref_does_not_exist "package1.ocu.star/@1/task/check_envs"

    echo "Test passed"
}


setup_test() {
    rm -rf .store
    rm -rf .ocuroot
}

build_ocuroot

pushd "$(dirname "$0")" > /dev/null
delete_environment
create_environment
create_environment_omnibus
popd > /dev/null
