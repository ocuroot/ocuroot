ocuroot("0.3.0")

def build(ctx):
    print("Building")
    done()

phase(
    name="build",
    work=[call(build, name="build")],
)

phase(
    name="build2",
    work=[call(build, name="build2")],
)