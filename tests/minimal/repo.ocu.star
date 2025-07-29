ocuroot("0.3.0")

repo_alias("minimal/repo")

store.set(
    store.fs(".store/state"),
    #store.git("ssh://git@github.com/ocuroot/statetest.git"),
    intent=store.fs(".store/intent"),
)

def do_trigger(commit):
    print("Triggering work for repo at commit " + commit)

    host.shell(
        "cd ~/src/github.com/ocuroot/ocuroot/examples/minimal && ocuroot work continue",
        env = {
            "OCU_REPO_COMMIT_OVERRIDE": commit,
        },
    )

trigger(do_trigger)