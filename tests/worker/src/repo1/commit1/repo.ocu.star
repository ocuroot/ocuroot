ocuroot("0.3.0")

repo_alias = "push"

env_vars = host.env()

state_store = store.git(
    env_vars["REPO_REMOTE"],
    branch="state",
)

intent_store = store.git(
    env_vars["REPO_REMOTE"], 
    branch="intent",
)