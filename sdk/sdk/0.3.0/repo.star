def repo_alias(alias):
    """
    repo_alias registers the alias for the current repository.
    This alias will be used to refer to the repository instead of the remote URL.

    Args:
        alias: The alias to register
    """
    backend.repo.alias(json.encode(alias))


def trigger(fn):
    """
    trigger allows you to define a function to trigger work against this repository
    The provided function should cause `ocuroot work continue` to be called in this repository
    at the correct commit.

    Args:
        fn: The function to trigger. It should take a commit ref as its only parameter.
    """
    backend.repo.trigger(fn)