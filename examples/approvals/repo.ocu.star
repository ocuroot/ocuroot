ocuroot("0.3.0")

# An alias for the repository, this replaces the git URI
repo_alias("approvals")

# Configure state storage on the filesystem
store.set(
    store.fs(".store/state"),
    intent=store.fs(".store/intent"),
)