ocuroot("0.3.14")

repo_alias("sdk_version")

store.set(
    store.fs(".store/state"),
    intent=store.fs(".store/intent"),
)