ocuroot("0.3.0")

def up(environment, message):
    return done(
        outputs={
            "message": message,
        }
    )

def down(environment, message):
    return done()

phase(
    name="staging",
    tasks=[
        deploy(
            up=up,
            down=down,
            environment=environment,
            inputs={
                "message": input(
                    ref="cascade/-/a.ocu.star/@/deploy/{}#output/message".format(environment.name),
                ),
            },
        ) for environment in environments()
    ],
)