ocuroot("0.3.0")

repo_alias("minimal/repo")

store.set(
    store.fs(".store/state"),
    #store.git("ssh://git@github.com/ocuroot/statetest.git"),
    intent=store.fs(".store/intent"),
)
