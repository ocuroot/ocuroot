ocuroot("0.3.0")

def up(ctx):
    # The first attempt to run this function will fail, as ready.txt does not exist
    # It will always succeed after this first attempt.
    # This simulates a failure that can be retried.
    if host.shell("test -f .data/ready.txt", continue_on_error=True).exit_code != 0:
        host.shell("mkdir -p .data")
        host.shell("touch .data/ready.txt")
        fail("first run, creating ready.txt and exiting")

    return done()

def down(ctx):
    return done()

phase(
    "release",
    work=[
        deploy(
            up=up,
            down=down,
            environment=environment,
        ) for environment in environments() 
    ],
)

def postrelease(ctx):
    return done(
        outputs={
            "foo": "bar",
        }
    )

phase(
    "postrelease",
    work=[
        call(
            fn=postrelease,
            name="postrelease",
        )
    ],
)

