ocuroot("0.3.0")

def up(ctx):
    print("package2 up")
    write("./.deploys/{}/package2.txt".format(ctx.inputs.environment["name"]), str(ctx))
    return done()

def down(ctx):
    print("package2 down")
    shell("rm -f ./.deploys/{}/package2.txt".format(ctx.inputs.environment["name"]))
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