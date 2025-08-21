#!/bin/bash

# Default port for the CI server
CI_PORT=8081
# Track the CI server process ID
CI_SERVER_PID=""

REL_DIR="$(dirname "$0")"
TEST_CONTENT_DIR="$(readlink -f "$REL_DIR")"

cleanup_dangling_ci() {
    # Kill any minici processes that might be running
    pkill -f "minici" 2>/dev/null || true

    if lsof -Pi :$CI_PORT -t >/dev/null; then
        echo "There is a process listening on port $CI_PORT but it is not minici, command is:"
        lsof -Pi :$CI_PORT -t | xargs ps -o comm= -p
        exit 1
    fi
}

# Build and start the CI server
start_ci_server() {    
    echo "Starting CI server on port $CI_PORT in background..."
    go run github.com/ocuroot/minici/cmd/minici@v0.0.1 -port "$CI_PORT" &
    CI_SERVER_PID=$!
    
    # Give the server a moment to start up
    START_TIME=$(date +%s.%N)
    while ! lsof -Pi :$CI_PORT -t >/dev/null; do
        NOW=$(date +%s.%N)
        if [ $(echo "$NOW - $START_TIME > 30" | bc -l) -eq 1 ]; then
            echo "CI server failed to start within 30s"
            exit 1
        fi
        sleep 0.01
    done
    END_TIME=$(date +%s.%N)
    DELTA=$(echo "$END_TIME - $START_TIME" | bc -l)
    echo "CI server took $DELTA seconds to start up"
    
    echo "CI server running with PID: $CI_SERVER_PID"
}

# Cleanup function to stop the CI server and remove the binary
cleanup_ci_server() {
    if [ -n "$CI_SERVER_PID" ]; then
        echo "Stopping CI server (PID: $CI_SERVER_PID) and all child processes..."
        
        # Kill the entire process group
        kill -TERM -$CI_SERVER_PID 2>/dev/null || true
        
        # Give processes time to terminate gracefully
        sleep 2
        
        # Force kill if still running
        kill -KILL -$CI_SERVER_PID 2>/dev/null || true
        
        CI_SERVER_PID=""
    fi
    
    # Kill any remaining minici processes
    pkill -f "minici" 2>/dev/null || true
    
    # Give it a moment to clean up
    sleep 1
    
    # Clean up test repositories
    if [ -d "$TEST_REPO_DIR" ]; then
        echo "Cleaning up test repositories..."
        rm -rf "$TEST_REPO_DIR"
    fi
}

create_repo() {
    # Create a temporary directory for the test
    TEST_REPO_DIR=$(mktemp -d)

    pushd "$TEST_REPO_DIR" > /dev/null
    
    # Create a bare git repository
    echo "Creating bare repository..." >&2
    mkdir -p "repo.git"
    pushd "repo.git" > /dev/null
    git -c init.defaultBranch=$DEFAULT_BRANCH_NAME init --bare >&2
    assert_equal "0" "$?" "Failed to initialize bare repository"
    popd > /dev/null
    
    # Clone the repository for working
    echo "Cloning working repository..." >&2
    git clone "$TEST_REPO_DIR/repo.git" "working" >&2
    assert_equal "0" "$?" "Failed to clone repository"
    
    # Create state and intent branches in the bare repository
    echo "Creating state and intent branches..." >&2
    pushd "repo.git" > /dev/null
    # Create empty commits for state and intent branches
    # First create an empty tree
    EMPTY_TREE=$(git hash-object -t tree /dev/null)
    # Create empty commits pointing to this tree
    STATE_COMMIT=$(git commit-tree $EMPTY_TREE -m "Initial empty state commit")
    git update-ref refs/heads/state $STATE_COMMIT >&2

    INTENT_COMMIT=$(git commit-tree $EMPTY_TREE -m "Initial empty intent commit")
    git update-ref refs/heads/intent $INTENT_COMMIT >&2
    
    popd > /dev/null
    
    pushd "working" > /dev/null
    
    # Set up Git configuration for the commit
    git config user.email "test@ocuroot.com"
    git config user.name "Test User"
    
    # Copy the repo directory from triggers to the working repo
    echo "Copying test repository files..." >&2
    # Use absolute path to the repo directory
    REPO_DIR="$TEST_CONTENT_DIR/repo"
    echo "Copying files from $REPO_DIR" >&2
    cp -R "$REPO_DIR"/* ./
    
    # Add files and commit
    echo "Committing repository files..." >&2
    git add . >&2
    git commit -m "Initial commit with ocuroot package files" >&2
    assert_equal "0" "$?" "Failed to commit files"
    
    # Push to the bare repository
    CURRENT_BRANCH=$(git branch --show-current)
    echo $CURRENT_BRANCH >&2
    git push origin "$CURRENT_BRANCH:$CURRENT_BRANCH" >&2
    assert_equal "0" "$?" "Failed to push to repository"
    
    # Get the commit hash
    COMMIT_HASH=$(git rev-parse HEAD)
    
    popd > /dev/null
    popd > /dev/null

    echo $TEST_REPO_DIR
}

checkout_repo() {
    local repo_uri=$1
    
    local repo_dir=$(mktemp -d)
    pushd "$repo_dir" > /dev/null
    git clone "$repo_uri" . >&2
    popd > /dev/null
    echo "$repo_dir"
}

checkout_and_modify_repo() {
    local repo_uri=$1
    local file_path=$2
    local new_content=$3
    
    local repo_dir=$(checkout_repo "$repo_uri")
    pushd "$repo_dir" > /dev/null
    
    echo "$new_content" > "$file_path"
    git add "$file_path"
    git commit -m "Test commit modifying $file_path" >&2
    assert_equal "0" "$?" "Failed to commit file"
    
    CURRENT_BRANCH=$(git branch --show-current)
    echo $CURRENT_BRANCH >&2
    git push origin "$CURRENT_BRANCH:$CURRENT_BRANCH" >&2
    assert_equal "0" "$?" "Failed to push to repository"
    
    popd > /dev/null
    echo "$repo_dir"
}


job_count() {
    local jobs_response=$(curl -s "http://localhost:$CI_PORT/api/jobs")
    echo $jobs_response | jq -r '.jobs | length'
}

# To iterate over the list of job IDs:
# for job_id in $(job_ids); do
#     echo "Job ID: $job_id"
# done
job_ids() {
    local jobs_response=$(curl -s "http://localhost:$CI_PORT/api/jobs")
    echo $jobs_response | jq -r '.jobs[]'
}

job_logs() {
    local job_id=$1
    local logs_response=$(curl -s "http://localhost:$CI_PORT/api/jobs/$job_id/logs")
    echo $logs_response | jq -r '.logs[]'
}

all_job_logs() {
    local jobs_response=$(curl -s "http://localhost:$CI_PORT/api/jobs")
    for job_id in $(echo $jobs_response | jq -r '.jobs[]'); do
        echo "Logs for job $job_id:"
        job_logs "$job_id"
    done
}

job_status() {
    local job_id=$1
    local job_response=$(curl -s "http://localhost:$CI_PORT/api/jobs/$job_id")
    echo $job_response | jq -r '.status'
}

job_detail() {
    local job_id=$1
    local job_response=$(curl -s "http://localhost:$CI_PORT/api/jobs/$job_id")
    echo $job_response | jq -r '.'
}

wait_for_all_jobs() {
    curl -s "http://localhost:$CI_PORT/api/wait"
    assert_equal "0" "$?" "Failed to wait for all jobs with CURL"
}

schedule_job() {
    local repo_uri=$1
    local commit=$2
    local command=$3

    local job_request='{"repo_uri":"'"$repo_uri"'","commit":"'"$commit"'","command":"'"$command"'"}'
    
    # Send job request to the server
    local response=$(curl -s -X POST -H "Content-Type: application/json" \
                       -d "$job_request" \
                       "http://localhost:$CI_PORT/api/jobs")
    
    # Extract job ID from response
    local job_id=$(echo $response | jq -r '.id')
    
    # Verify that we got a job ID
    if [ -z "$job_id" ] || [ "$job_id" == "null" ]; then
        echo "Failed to schedule job: $response" >&2
        exit 1
    else
        echo $job_id
    fi
}