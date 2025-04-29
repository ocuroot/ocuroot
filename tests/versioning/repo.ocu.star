ocuroot("0.3.0")

repo_alias("versioning")

store.set(
    store.fs(".store/state"),
    intent=store.fs(".store/intent"),
)
