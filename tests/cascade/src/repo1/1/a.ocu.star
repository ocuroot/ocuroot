ocuroot("0.3.0")

def up(environment):
    return done(
        outputs={
            "message": read("a.txt"),
        }
    )

def down(environment):
    return done()

phase(
    name="staging",
    tasks=[
        deploy(
            up=up,
            down=down,
            environment=environment,
        ) for environment in environments()
    ],
)