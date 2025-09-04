ocuroot("0.3.0")

envs = environments()
staging = [e for e in envs if e.attributes["type"] == "staging"]
prod = [e for e in envs if e.attributes["type"] == "prod"]

def build(ctx):
    print("Building")
    res = host.shell("cat message-backend.txt", mute=True)
    print("Message: {}".format(res.stdout))
    
    return done(
        outputs={
            "message": res.stdout,
        },
    )

def up(ctx):
    print("Deploying to {}".format(ctx.inputs.environment["name"]))
    return done(
        outputs={
            "message": ctx.inputs.message,
            "foo": "bar",
        },
    )

def down(ctx):
    print("Undeploying from {}".format(ctx.inputs.environment["name"]))
    return done()

phase(
    name="build",
    work=[call(build, name="build")],
)

phase(
    name="staging",
    work=[
        deploy(
            up=up,
            down=down,
            environment=environment,
            inputs={
                "message": input(ref="./task/build/#output/message"),
            },
        ) for environment in staging
    ],
)

phase(
    name="prod",
    work=[
        deploy(
            up=up,
            down=down,
            environment=environment,
            inputs={
                "message": input(ref="./task/build/#output/message"),
            },
        ) for environment in prod
    ],
)
