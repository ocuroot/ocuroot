ocuroot("0.3.0")

def build(ctx):
    print("Building minimal package")
    print(ctx)

    res = host.shell("pwd", mute=True)
    print("Current directory: ", res.stdout)

    print("host information:")
    print("OS: ", host.os())
    print("Arch: ", host.arch())

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
    outputs["environment"] = ctx.inputs.environment
    outputs["input1"] = ctx.inputs.input1

    # Output some log lines and force a pause
    for i in range(1,5):
        host.shell("sleep 0.01")
        print("Starlark print: " + str(i))

    return next(up_stage2, inputs=outputs)

def up_stage2(ctx):
    print("Continuing deploy to ", ctx.inputs.environment["name"])
    print(ctx.inputs.input1)
    return done(outputs={"env_name": ctx.inputs.environment["name"], "count": ctx.inputs.count}, tags=["v"+str(ctx.inputs.count)])

def down(ctx):
    print("Destroying minimal package")
    print(ctx)
    print("Environment: " + ctx.inputs.environment["name"])
    return done()