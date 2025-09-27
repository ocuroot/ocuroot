ocuroot("0.3.0")

repo_alias("cascade")

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