repo_alias("minimal/repo")

store.set(
    store.fs(".store/state"),
    intent=store.fs(".store/intent"),
)

def do_trigger(commit):
    print("Triggering work for repo at commit " + commit)

    host.shell(
        "cd ~/src/github.com/ocuroot/ocuroot/tests/minimal && ocuroot work continue",
        env = {
            "OCU_REPO_COMMIT_OVERRIDE": commit,
        },
    )

trigger(do_trigger)