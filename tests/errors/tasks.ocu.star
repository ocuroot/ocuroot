ocuroot("0.3.0")

def build(ctx):
    print("Building minimal package")
    print(ctx)

    res = host.shell("pwd", mute=True)
    print("Current directory: ", res.stdout.strip())

    print("host information:")
    print("OS: ", host.os())
    print("Arch: ", host.arch())

    for i in range(1,10):
        host.shell("sleep 0.1")
        print("Log line number " + str(i))

    return done(
        outputs={
            "output1": 5.5,
            "output2": "value2",
            "output3": True,
            "output4": 3,
        },
    )

def up(ctx):
    print("Deploying minimal package")    
    print("ctx: ", ctx)
    print("Environment: ", ctx.inputs.environment["name"])
    outputs = {}
    outputs["count"] = ctx.inputs.previous_count + 1
    outputs["env_name"] = ctx.inputs.environment
    outputs["input1"] = ctx.inputs.input1

    # Output some log lines and force a pause
    for i in range(1,10):
        host.shell("sleep 0.1")
        print("Log line number " + str(i))

    if ctx.inputs.environment["name"] == "production2":
        fail("something is wrong in production2")

    return done(outputs=outputs)

def down(ctx):
    print("Destroying minimal package")
    print(ctx)
    return done()