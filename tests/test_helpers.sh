#!/usr/bin/env bash

build_ocuroot() {
    if [ -n "$NO_INSTALL" ]; then
        return 0
    fi

    pushd "$(dirname "$0")/../../" > /dev/null
    make install
    assert_equal "0" "$?" "Failed to install ocuroot"
    popd > /dev/null
}

assert_file_exists() {
    local file_path="$1"
    local error_message="${2:-"File $file_path does not exist"}"

    if [ ! -f "$file_path" ]; then
        echo "$error_message"
        exit 1
    fi
    return 0
}

assert_ref_equals() {
    local ref_path="$1"
    local expected_value="$2"
    local raw_value=$(ocuroot state get "$ref_path" 2>/dev/null)
    local actual_value=$(echo "$raw_value" | jq -r '.')
    local error_message="${3:-"Ref $ref_path does not match expected value, expected $expected_value, got $actual_value\n\n$raw_value"}"

    if [ "$actual_value" != "$expected_value" ]; then
        echo "$error_message"
        exit 1
    fi
    return 0
}

# Function to check if a ref exists in state
# Usage: check_ref_exists "path/to/package.ocu.star/@/ref/path" "Error message if not found"
check_ref_exists() {
    local ref_path="$1"
    local error_message="${2:-"Ref $ref_path not found in state"}"

    ocuroot state get "$ref_path" > /dev/null 2> /dev/null
    if [ $? -ne 0 ]; then
        echo "$error_message"
        exit 1
    fi
    return 0
}

check_ref_does_not_exist() {
    local ref_path="$1"
    local error_message="${2:-"Ref $ref_path exists in state"}"

    ocuroot state get "$ref_path" > /dev/null 2> /dev/null
    if [ $? -eq 0 ]; then
        echo "$error_message"
        exit 1
    fi
    return 0
}

# Function to check if a package is deployed to an environment
# Usage: check_deployment "package/path.ocu.star" "environment"
assert_deployed() {
    local package_path="$1"
    local environment="$2"
    local ref_path="${package_path}/@/deploy/${environment}"
    local error_message="${3:-"${package_path} not deployed to ${environment}"}"

    check_ref_exists "$ref_path" "$error_message"
    if [ $? -ne 0 ]; then
        exit 1
    fi
    return 0
}

assert_not_deployed() {
    local package_path="$1"
    local environment="$2"
    local ref_path="${package_path}/@/deploy/${environment}"
    local error_message="${3:-"${package_path} deployed to ${environment}"}"

    check_ref_does_not_exist "$ref_path" "$error_message"
    if [ $? -ne 0 ]; then
        exit 1
    fi
    return 0
}

assert_equal() {
    local expected="$1"
    local actual="$2"
    local error_message="${3:-"Expected $expected, got $actual"}"

    if [ "$expected" != "$actual" ]; then
        echo "$error_message"
        exit 1
    fi
    return 0
}

assert_not_equal() {
    local expected="$1"
    local actual="$2"
    local error_message="${3:-"Expected $expected, got $actual"}"

    if [ "$expected" = "$actual" ]; then
        echo "$error_message"
        exit 1
    fi
    return 0
}

