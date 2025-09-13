ocuroot("0.3.0")

repo_alias("gitstate/repo")

env_vars = host.env()

store.set(
    store.git(
        env_vars["STATE_REMOTE"],
        branch="main",
        support_files={"support.txt": "state"},
    )
)

def do_trigger(commit):
    host.shell("mkdir -p ./.triggers")
    host.shell("echo go > ./.triggers/{}".format(commit))

trigger(do_trigger)