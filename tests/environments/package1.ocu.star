ocuroot("0.3.0")

def up(ctx):
    print("package1 up")
    write("./.deploys/{}/package1.txt".format(ctx.inputs.environment["name"]), str(ctx))
    return done()

def down(ctx):
    print("package1 down")
    content = read("./.deploys/{}/package1.txt".format(ctx.inputs.environment["name"]))
    if content != str(ctx):
        return fail("content does not match")
    shell("rm -f ./.deploys/{}/package1.txt".format(ctx.inputs.environment["name"]))
    return done()

phase(
    "deploy",
    work=[
        deploy(
            up=up,
            down=down,
            environment=environment,
        ) for environment in environments()
    ]
)