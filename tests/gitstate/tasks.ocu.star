ocuroot("0.3.0")

def build():
    print("Building minimal package")

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

def up(environment, previous_count, input1, staging_name=""):
    print("Deploying minimal package")    
    print("Environment: ", environment["name"])
    outputs = {}
    outputs["count"] = previous_count + 1
    outputs["environment"] = environment
    outputs["input1"] = input1

    # Output some log lines and force a pause
    for i in range(1,5):
        host.shell("sleep 0.01")
        print("Starlark print: " + str(i))

    return next(up_stage2, inputs=outputs)

def up_stage2(count, environment, input1):
    print("Continuing deploy to ", environment["name"])
    print(input1)
    return done(outputs={"env_name": environment["name"], "count": count}, tags=["v"+str(count)])

def down(environment, previous_count, input1, staging_name=""):
    print("Destroying minimal package")
    print("Environment: " + environment["name"])
    return done()