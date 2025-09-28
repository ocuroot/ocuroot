ocuroot("0.3.0")

repo_alias("repo2")

env_vars = host.env()

store.set(
    store.git(
        env_vars["REPO_REMOTE"],
        branch="state",
    ),
    intent=store.git(
        env_vars["REPO_REMOTE"], 
        branch="intent",
    ),
)