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

def up(environment, credential):
    return done(outputs={
        "host": "{}.backend.example.com".format(environment["name"]),
        "credential": credential,
    })

def down(environment, credential):
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