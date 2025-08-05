#!/bin/bash


# Initialize git repositories and set up environment variables
init_repos() {
    local base_dir=${1:-$(mktemp -d)}
    
    # Export environment variables for use after function returns - only remotes
    export STATE_REMOTE="$base_dir/state.git"
    export INTENT_REMOTE="$base_dir/intent.git"
    
    # Create bare repositories
    git -c init.defaultBranch=main init --bare "$STATE_REMOTE"
    git -c init.defaultBranch=main init --bare "$INTENT_REMOTE"
    
    echo "Repositories initialized at:"
    echo "STATE_REMOTE: $STATE_REMOTE"
    echo "INTENT_REMOTE: $INTENT_REMOTE"
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