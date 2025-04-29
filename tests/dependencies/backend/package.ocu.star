ocuroot("0.3.0")

envs = environments()

def setup_deploy(environment):
    return deploy(
        up=up,
        down=down,
        environment=environment,
        inputs={
            "credential": input(ref="./@/custom/credential/{}".format(environment.name), default="abcd"),
        }
    )

def up(ctx):
    return done(outputs={
        "host": "{}.backend.example.com".format(ctx.inputs.environment["name"]),
        "credential": ctx.inputs.credential,
    })

def down(ctx):
    return done()

def build(ctx):
    return done()

phase(
    name="build",
    work=[call(fn=build, name="build")],
)

phase(
    name="staging",
    work=[
        setup_deploy(e) for e in envs if e.attributes["type"] == "staging"
    ],
)
        
phase(
    name="prod",
    work=[
        setup_deploy(e) for e in envs if e.attributes["type"] == "prod"
    ],
)