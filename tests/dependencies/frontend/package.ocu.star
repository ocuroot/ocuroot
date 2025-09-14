ocuroot("0.3.0")

envs = environments()

def setup_deploy(environment):
    return deploy(
        up=up,
        down=down,
        environment=environment,
        inputs={
            "backend_credential": ref("./-/backend/package.ocu.star/@/deploy/{}#output/credential".format(environment.name)),
        }
    )

def up(environment, backend_credential):
    print("up")
    return done(
        outputs={
            "host": "{}.frontend.example.com".format(environment["name"]),
            "backend_credential": backend_credential,
        }
    )

def down(environment, backend_credential):
    return done()

def build():
    return done()

phase(
    name="build",
    tasks=[task(fn=build, name="build")],
)
phase(
    name="staging",
    tasks=[
        setup_deploy(e) for e in envs if e.attributes["type"] == "staging"
    ],
)
phase(
    name="prod",
    tasks=[
        setup_deploy(e) for e in envs if e.attributes["type"] == "prod"
    ],
)