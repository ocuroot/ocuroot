ocuroot("0.3.14")

repo_alias("versioning")

store.set(
    store.fs(".store/state"),
    intent=store.fs(".store/intent"),
)
