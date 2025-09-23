#!/usr/bin/env bash


init_repo() {
    local base_dir=${1:-$(mktemp -d)}
    mkdir -p "$base_dir"
    export REPO_REMOTE="$(realpath $base_dir)/repo.git"
    git -c init.defaultBranch=main init --bare "$REPO_REMOTE"

    echo "Repository initialized at: $REPO_REMOTE"
}

# Initialize git repositories and set up environment variables
init_repos() {
    local base_dir=${1:-$(mktemp -d)}
    
    # Export environment variables for use after function returns - only remotes
    export STATE_REMOTE="$base_dir/state.git"
    export INTENT_REMOTE="$base_dir/intent.git"
    #export INTENT_REMOTE=$STATE_REMOTE
    
    # Create bare repositories
    git -c init.defaultBranch=main init --bare "$STATE_REMOTE"
    git -c init.defaultBranch=main init --bare "$INTENT_REMOTE"
    
    echo "Repositories initialized at:"
    echo "STATE_REMOTE: $STATE_REMOTE"
    echo "INTENT_REMOTE: $INTENT_REMOTE"
}

# Initialize the working directory with first commit
init_working_dir() {
    local working_dir_new=$1
    local remote_url=$2
    local var_name=$3
    
    # Create directory if it doesn't exist
    mkdir -p "$working_dir_new"
    local working_dir=$(realpath "$working_dir_new")
    
    # Export the working directory variable if a variable name is provided
    if [ -n "$var_name" ]; then
        export $var_name="$working_dir"
        echo "$var_name: $working_dir"
    fi

    # Initialize git repo
    git -c init.defaultBranch=main -C "$working_dir" init
    
    # Add remote
    git -C "$working_dir" remote add origin "$remote_url"
    
    # Configure git identity for the commit
    git -C "$working_dir" config user.name "Test Script"
    git -C "$working_dir" config user.email "test@example.com"
    
    # Create empty commit and push back to remote
    git -C "$working_dir" commit --allow-empty -m "Initial commit"
    git -C "$working_dir" push --set-upstream origin main
}


# Check if a file exists in a remote repo and has specific content
check_file_in_remote() {
    local remote_url="$1"
    local branch="$2"
    local file_path="$3"
    local expected_content="$4"
    
    local tmp_dir=$(mktemp -d)
    trap "rm -rf $tmp_dir" EXIT
    git clone "$remote_url" "$tmp_dir"
    git -C "$tmp_dir" checkout "$branch"
    local file_content=$(cat "$tmp_dir/$file_path")
    if [ "$file_content" == "$expected_content" ]; then
        return 0
    else
        echo "File content does not match expected content"
        exit 1
    fi
}

check_last_log_in_remote() {
    local remote_url="$1"
    local branch="$2"
    local expected_content="$3"
    
    local tmp_dir=$(mktemp -d)
    trap "rm -rf $tmp_dir" EXIT
    git clone "$remote_url" "$tmp_dir"
    git -C "$tmp_dir" checkout "$branch"
    local last_log=$(git -C "$tmp_dir" log -1)
    if echo "$last_log" | grep -q "$expected_content"; then
        return 0
    else
        echo "Last log does not contain expected content"
        echo "Got: "
        echo "$last_log"
        exit 1
    fi
}