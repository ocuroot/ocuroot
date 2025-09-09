def _set_store(state,intent=None):
    """
    Declares the state store to be used for releases.
    This should only be declared once, ideally in the repo.ocu.star file.

    Args:
        state: Storage for release and deployment states. May be specified using `store.git` or `store.fs`.
        intent: Storage for deployment intent. May be specified using `store.git` or `store.fs`. If not specified, intent will be kept in the state store.
    
    Example:
        store.set(store.git("ssh://git@github.com/example/state.git"))
        store.set(store.git("ssh://git@github.com/example/state.git"), intent=store.git("ssh://git@github.com/example/intent.git"))
    """
    backend.store.set(json.encode({"state": state, "intent": intent}))

def _git_store(remote_url, branch=None, create_branch=True, support_files=None):
    """
    Creates a git store for the given remote URL.
    
    Args:
        remote_url: The remote URL of the git repository
        branch: The branch containing the release state
        create_branch: If True, the branch will be created if it doesn't already exist. Defaults True.
        support_files: Files to add to the repository, as a dictionary of path to content, from the repository root
    
    Returns:
        A git store
    """
    return {
        "git": {
            "remote_url": remote_url,
            "branch": branch,
            "create_branch": create_branch,
            "support_files": support_files,
        }
    }

def _fs_store(path):
    """
    Creates a file system store for the given path.
    
    Args:
        path: The path to the directory where the store will be stored, relative to the repo root
    
    Returns:
        A file system store
    """
    return {
        "fs": {
            "path": path,
        }
    }

store = struct(
    set = _set_store,
    git = _git_store,
    fs = _fs_store,
)