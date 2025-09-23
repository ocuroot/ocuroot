ocuroot("0.3.0")

repo_alias("cascade")

env_vars = host.env()

store.set(
    store.git(
        env_vars["REPO_REMOTE"],
        branch="state",
        support_files={"support.txt": "state"},
    ),
    intent=store.git(
        env_vars["REPO_REMOTE"], 
        branch="intent",
        support_files={"support.txt": "intent"},
    ),
)

def do_trigger(commit):
    host.shell("mkdir -p ./.triggers")
    host.shell("echo go > ./.triggers/{}".format(commit))

trigger(do_trigger)