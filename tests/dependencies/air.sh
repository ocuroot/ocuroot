#!/bin/bash

OCU_REPO_COMMIT_OVERRIDE=commitid

pushd "$(dirname "$0")" > /dev/null
../../tmp/ocuroot $@
popd > /dev/null