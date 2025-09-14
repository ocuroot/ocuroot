ocuroot("0.3.0")

envs = environments()
staging = [e for e in envs if e.attributes["type"] == "staging"]
prod = [e for e in envs if e.attributes["type"] == "prod"]

def build():
    print("Building")
    return done()

def up(environment, foo):
    print("Deploying to {}".format(environment["name"]))
    print("foo: {}".format(foo))
    return done(
        outputs={
            "foo": foo,
        }
    )

def down(environment, foo):
    print("Undeploying from {}".format(environment["name"]))
    return done()

phase(
    name="build",
    tasks=[task(build, name="build")],
)

phase(
    name="staging",
    tasks=[
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
    tasks=[
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
