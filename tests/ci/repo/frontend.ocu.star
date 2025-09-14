ocuroot("0.3.0")

envs = environments()
staging = [e for e in envs if e.attributes["type"] == "staging"]
prod = [e for e in envs if e.attributes["type"] == "prod"]

def build():
    print("Building")
    res = host.shell("cat message-frontend.txt", mute=True)
    print("Message: {}".format(res.stdout))
    
    return done(
        outputs={
            "message": res.stdout,
        },
    )

def up(environment, foo, message, backend_message):
    print("Deploying to {}".format(environment["name"]))
    print("foo = {}".format(foo))
    print("Message: {}".format(message))
    return done(
        outputs={
            "backend_message": backend_message,
            "message": message,
        },
    )

def down(environment, foo, message, backend_message):
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
                "foo": input(ref="./-/backend.ocu.star/@/deploy/{environment}/#output/foo".format(environment=environment.name)),
                "message": input(ref="./task/build/#output/message"),
                "backend_message": input(ref="./-/backend.ocu.star/@/deploy/{environment}/#output/message".format(environment=environment.name)),
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
                "foo": input(ref="./-/backend.ocu.star/@/deploy/{environment}/#output/foo".format(environment=environment.name)),
                "message": input(ref="./task/build/#output/message"),
                "backend_message": input(ref="./-/backend.ocu.star/@/deploy/{environment}/#output/message".format(environment=environment.name)),
            },
        ) for environment in prod
    ],
)
