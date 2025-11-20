#!/usr/bin/env bash

make test-build
if [ $? -ne 0 ]; then
    echo "Test build failed"
    exit 1
fi

NO_INSTALL=1 ./tests/minimal/test.sh
if [ $? -ne 0 ]; then
    echo "Minimal test failed"
    exit 1
fi

NO_INSTALL=1 ./tests/dependencies/test.sh
if [ $? -ne 0 ]; then
    echo "Dependencies test failed"
    exit 1
fi

NO_INSTALL=1 ./tests/errors/test.sh
if [ $? -ne 0 ]; then
    echo "Errors test failed"
    exit 1
fi

NO_INSTALL=1 ./tests/versioning/test.sh
if [ $? -ne 0 ]; then
    echo "Versioning test failed"
    exit 1
fi

NO_INSTALL=1 ./tests/secrets/test.sh
if [ $? -ne 0 ]; then
    echo "Secrets test failed"
    exit 1
fi

NO_INSTALL=1 ./tests/environments/test.sh
if [ $? -ne 0 ]; then
    echo "Environments test failed"
    exit 1
fi

NO_INSTALL=1 ./tests/retries/test.sh
if [ $? -ne 0 ]; then
    echo "Retries test failed"
    exit 1
fi

NO_INSTALL=1 ./tests/validation/test.sh
if [ $? -ne 0 ]; then
    echo "Validation test failed"
    exit 1
fi

NO_INSTALL=1 ./tests/customstate/test.sh
if [ $? -ne 0 ]; then
    echo "Customstate test failed"
    exit 1
fi

NO_INSTALL=1 ./tests/sdk_version/test.sh
if [ $? -ne 0 ]; then
    echo "SDK version test failed"
    exit 1
fi

NO_INSTALL=1 ./tests/gitstate/test.sh
if [ $? -ne 0 ]; then
    echo "Gitstate test failed"
    exit 1
fi

NO_INSTALL=1 ./tests/gitstate_shared/test.sh
if [ $? -ne 0 ]; then
    echo "Gitstate shared test failed"
    exit 1
fi

NO_INSTALL=1 ./tests/cascade/test.sh
if [ $? -ne 0 ]; then
    echo "Cascade test failed"
    exit 1
fi

NO_INSTALL=1 ./tests/push/test.sh
if [ $? -ne 0 ]; then
    echo "Push test failed"
    exit 1
fi

NO_INSTALL=1 ./tests/worker/test.sh
if [ $? -ne 0 ]; then
    echo "Worker test failed"
    exit 1
fi

NO_INSTALL=1 ./tests/ci/test.sh
if [ $? -ne 0 ]; then
    echo "CI test failed"
    exit 1
fi