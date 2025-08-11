ocuroot("0.3.0")

envs = environments()
staging = [e for e in envs if e.attributes["type"] == "staging"]
prod = [e for e in envs if e.attributes["type"] == "prod"]

def build(ctx):
    print("Building")
    return done()

def up(ctx):
    print("Deploying to {}".format(ctx.inputs.environment["name"]))
    print("foo: {}".format(ctx.inputs.foo))
    return done(
        outputs={
            "foo": ctx.inputs.foo,
        }
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
                "foo": input(ref="@/custom/foo", default="bar"),
            }
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
                "foo": input(ref="@/custom/foo", default="bar"),
            }
        ) for environment in prod
    ],
)
