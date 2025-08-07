ocuroot("0.3.0")

def up(ctx):
    return done()

def down(ctx):
    return done()

phase(
    "deploy",
    work=[
        deploy(
            up=up,
            down=down,
            environment=environment,
        ) for environment in environments()
    ]
)