#!/usr/bin/env bash

export OCUROOT_HOME=$(pwd)/$(dirname "$0")/.ocuroot

source $(dirname "$0")/../test_helpers.sh

build_ocuroot

pushd "$(dirname "$0")" > /dev/null

rm -rf .store
rm -rf .ocuroot

export OCU_REPO_COMMIT_OVERRIDE=commit1
echo "== release 0.1.0-1 =="
ocuroot release new release.ocu.star
echo "== create promotion intent =="
ocuroot state set "versioning/-/release.ocu.star/@0.1.0-1/custom/promote" 1
echo "== apply promotion intent =="
ocuroot state apply "versioning/-/release.ocu.star/@0.1.0-1/custom/promote"
echo "== continue release =="
ocuroot release continue versioning/-/release.ocu.star/@0.1.0-1

export OCU_REPO_COMMIT_OVERRIDE=commit2
echo "== release 0.1.1-1 =="
ocuroot release new release.ocu.star
# Stops here

export OCU_REPO_COMMIT_OVERRIDE=commit3
echo "== release 0.1.1-2 =="
ocuroot release new release.ocu.star
echo "== create promotion intent =="
ocuroot state set "versioning/-/release.ocu.star/@0.1.1-2/custom/promote" 1
echo "== apply promotion intent =="
ocuroot state apply "versioning/-/release.ocu.star/@0.1.1-2/custom/promote"
echo "== continue release =="
ocuroot release continue versioning/-/release.ocu.star/@0.1.1-2

export OCU_REPO_COMMIT_OVERRIDE=commit4
echo "== release 0.1.2-1 =="
ocuroot release new release.ocu.star

echo "Test succeeded"
echo ""

popd > /dev/null
