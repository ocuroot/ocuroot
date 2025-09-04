ocuroot("0.3.0")

envs = environments()

def setup_deploy(environment):
    return deploy(
        up=up,
        down=down,
        environment=environment,
        inputs={
            "backend_host": ref("./-/backend/package.ocu.star/@/deploy/{}#output/host".format(environment.name)),
            "backend_credential": ref("./-/backend/package.ocu.star/@/deploy/{}#output/credential".format(environment.name)),
        }
    )

def up(ctx):
    print("up")
    print(ctx)
    return done(
        outputs={
            "host": "{}.frontend.example.com".format(ctx.inputs.environment["name"]),
            "backend_credential": ctx.inputs.backend_credential,
        }
    )

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