#!/usr/bin/env bash

export OCU_REPO_COMMIT_OVERRIDE=${OCU_REPO_COMMIT_OVERRIDE:-commitid}

source $(dirname "$0")/../test_helpers.sh

build_ocuroot

pushd "$(dirname "$0")" > /dev/null

rm -rf .store

echo "== set up environments =="
ocuroot release new environments/package.ocu.star
echo "== release package =="
ocuroot release new package.ocu.star
assert_equal "1" "$?" ""

echo "Test succeeded"

popd > /dev/null
