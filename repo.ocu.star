ocuroot("0.3.0")

repo_alias("github.com/ocuroot/ocuroot")

store.set(
    store.git("ssh://git@github.com/ocuroot/ocuroot-state.git"),
    intent=store.git("ssh://git@github.com/ocuroot/ocuroot-intent.git"),
)