ocuroot("0.3.0")

def build():
    print("Building minimal package")

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

def up(environment, previous_count, input1, staging_name=""):
    print("Deploying minimal package")    
    print("Environment: ", environment["name"])
    outputs = {}
    outputs["count"] = previous_count + 1
    outputs["env_name"] = environment
    outputs["input1"] = input1

    # Output some log lines and force a pause
    for i in range(1,10):
        host.shell("sleep 0.1")
        print("Log line number " + str(i))

    if environment["name"] == "production2":
        fail("something is wrong in production2")

    return done(outputs=outputs)

def down(environment, previous_count, input1, staging_name=""):
    print("Destroying minimal package")
    return done()