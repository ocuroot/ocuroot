#!/usr/bin/env bash


# Initialize git repositories and set up environment variables
init_repos() {
    local base_dir=${1:-$(mktemp -d)}
    
    # Export environment variables for use after function returns - only remotes
    export STATE_REMOTE="$base_dir/state.git"
    
    # Create bare repositories
    git -c init.defaultBranch=main init --bare "$STATE_REMOTE"
    
    echo "Repositories initialized at:"
    echo "STATE_REMOTE: $STATE_REMOTE"
}

# Initialize the working directory with first commit
init_working_dir() {
    local working_dir=${1:-$(mktemp -d)}
    local remote_url=$2
    local var_name=$3
    
    # Export the working directory variable if a variable name is provided
    if [ -n "$var_name" ]; then
        export $var_name="$working_dir"
        echo "$var_name: $working_dir"
    fi
    
    # Create directory if it doesn't exist
    mkdir -p "$working_dir"
    
    # Initialize git repo
    git -c init.defaultBranch=main -C "$working_dir" init
    
    # Add remote
    git -C "$working_dir" remote add origin "$remote_url"
    
    # Configure git identity for the commit
    git -C "$working_dir" config user.name "Test Script"
    git -C "$working_dir" config user.email "test@example.com"
    
    # Create empty commit and push back to remote
    git -C "$working_dir" commit --allow-empty -m "Initial commit"
    git -C "$working_dir" push origin HEAD:main
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