ocuroot("0.3.0")

repo_alias("gitstate/repo")

env_vars = host.env()

def setup_store():
    print("Setting up store")
    git_remote_res = host.shell("git remote get-url origin", continue_on_error=True)
    git_remote = git_remote_res.stdout.strip()
    if git_remote_res.exit_code != 0 or git_remote == "" or git_remote.startswith("git@"):
        print("Using filesystem store")
        store.set(store.fs("./.state"))
    else:
        print("Using git store:", git_remote)
        state_store = store.git(git_remote, branch="state")
        print("State store:", state_store)
        intent_store = store.git(git_remote, branch="intent")
        print("Intent store:", intent_store)
        store.set(state_store, intent=intent_store)

def do_trigger(commit):
    print("triggering for commit:" + commit)
    git_remote_res = host.shell("git remote get-url origin", continue_on_error=True)
    git_remote = git_remote_res.stdout.strip()
    http.post("http://localhost:8081/api/jobs", body=json.encode({"repo_uri": git_remote, "commit": commit, "command": "./continue.sh"}))

setup_store()
trigger(do_trigger)