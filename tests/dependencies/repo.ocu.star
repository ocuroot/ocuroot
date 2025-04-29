ocuroot("0.3.0")

repo_alias("dependencies")

store.set(
    store.fs(".store/state"),
    intent=store.fs(".store/intent"),
)

def do_trigger(commit):
    host.shell("mkdir -p ./.store/triggers")
    host.shell("echo go > ./.store/triggers/{}".format(commit))

trigger(do_trigger)
