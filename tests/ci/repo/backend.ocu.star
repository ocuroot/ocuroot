ocuroot("0.3.0")

envs = environments()
staging = [e for e in envs if e.attributes["type"] == "staging"]
prod = [e for e in envs if e.attributes["type"] == "prod"]

def build():
    print("Building")
    res = host.shell("cat message-backend.txt", mute=True)
    print("Message: {}".format(res.stdout))
    
    return done(
        outputs={
            "message": res.stdout,
        },
    )

def up(environment, message):
    print("Deploying to {}".format(environment["name"]))
    return done(
        outputs={
            "message": message,
            "foo": "bar",
        },
    )

def down(environment, message):
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
                "message": input(ref="./task/build/#output/message"),
            },
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
                "message": input(ref="./task/build/#output/message"),
            },
        ) for environment in prod
    ],
)
