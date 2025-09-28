ocuroot("0.3.0")

repo_alias("environments")

store.set(
    store.fs(".store/state"),
    intent=store.fs(".store/intent"),
)

# Recursively trigger additional work
# Limited to 5 levels of recursion
def _trigger(commit):
    if "TRIGGER_DEPTH" in host.env():
        trigger_depth = int(host.env()["TRIGGER_DEPTH"])
    else:
        trigger_depth = 5

    if trigger_depth <= 0:
        fail("Trigger depth exceeded")

    print("triggering for commit:" + commit)
    host.shell("ocuroot work any", env={"TRIGGER_DEPTH": str(trigger_depth - 1)})

trigger(_trigger)
