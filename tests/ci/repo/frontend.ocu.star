ocuroot("0.3.0")

envs = environments()
staging = [e for e in envs if e.attributes["type"] == "staging"]
prod = [e for e in envs if e.attributes["type"] == "prod"]

def build(ctx):
    print("Building")
    res = host.shell("cat message-frontend.txt", mute=True)
    print("Message: {}".format(res.stdout))
    
    return done(
        outputs={
            "message": res.stdout,
        },
    )

def up(ctx):
    print("Deploying to {}".format(ctx.inputs.environment["name"]))
    print("foo = {}".format(ctx.inputs.foo))
    print("Message: {}".format(ctx.inputs.message))
    return done(
        outputs={
            "backend_message": ctx.inputs.backend_message,
            "message": ctx.inputs.message,
        },
    )

def down(ctx):
    print("Undeploying from {}".format(ctx.inputs.environment["name"]))
    print("Message: {}".format(ctx.inputs.message))
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
                "foo": input(ref="./-/backend.ocu.star/@/deploy/{environment}/#output/foo".format(environment=environment.name)),
                "message": input(ref="./task/build/#output/message"),
                "backend_message": input(ref="./-/backend.ocu.star/@/deploy/{environment}/#output/message".format(environment=environment.name)),
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
                "foo": input(ref="./-/backend.ocu.star/@/deploy/{environment}/#output/foo".format(environment=environment.name)),
                "message": input(ref="./task/build/#output/message"),
                "backend_message": input(ref="./-/backend.ocu.star/@/deploy/{environment}/#output/message".format(environment=environment.name)),
            },
        ) for environment in prod
    ],
)
