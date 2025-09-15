#!/usr/bin/env bash

export OCU_REPO_COMMIT_OVERRIDE=${OCU_REPO_COMMIT_OVERRIDE:-commitid}
export OCUROOT_HOME=$(pwd)/$(dirname "$0")/.ocuroot

source $(dirname "$0")/../test_helpers.sh

build_ocuroot

pushd "$(dirname "$0")" > /dev/null

rm -rf .store
rm -rf .ocuroot

echo "== create custom state =="
ocuroot state set @/custom/test 1
assert_equal "0" "$?" ""

echo "== apply custom state =="
ocuroot state apply @/custom/test
assert_equal "0" "$?" ""

check_ref_exists "@/custom/test"

echo "== delete custom state =="
ocuroot state delete @/custom/test
assert_equal "0" "$?" ""

echo "== apply deletion =="
ocuroot state apply @/custom/test
assert_equal "0" "$?" ""

check_ref_does_not_exist "@/custom/test"   

echo "Test succeeded"

popd > /dev/null
