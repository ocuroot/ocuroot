repo_alias("minimal/repo")

state_store = store.fs("./.store/state")
intent_store = store.fs("./.store/intent")

# state_store = store.local("minimal_state")
# intent_store = store.local("minimal_intent")

def do_trigger(commit):
    print("Triggering work for repo at commit " + commit)

    host.shell(
        "cd ~/src/github.com/ocuroot/ocuroot/tests/minimal && ocuroot work continue",
        env = {
            "OCU_REPO_COMMIT_OVERRIDE": commit,
        },
    )

trigger(do_trigger)