ocuroot("0.3.0")

# An example build function
# Shows some information and returns a set of exampel outputs
def build():
    print("Building minimal package")
    
    res = shell("pwd", mute=True)
    print("Current directory: ", res.stdout)

    print("host information:")
    print("OS: ", os())
    print("Arch: ", arch())

    return done(
        outputs={
            "output1": 5.5,
            "output2": "value2",
            "output3": True,
            "output4": 3,
        },
    )

# A deployment function that relies on the outputs from the build function
def up(environment, input1, previous_count=0):
    print("Deploying minimal package")    
    print("Environment: ", environment["name"])
    outputs = {}
    outputs["count"] = previous_count + 1
    outputs["environment"] = environment
    outputs["input1"] = input1

    # Output some log lines and force a pause
    for i in range(1,5):
        shell("sleep 0.01")
        print("Starlark print: " + str(i))

    # Hand off to another function, passing on the outputs
    return next(up_stage2, inputs=outputs)

# A second stage of the deployment function
def up_stage2(count, environment, input1):
    print("Continuing deploy to ", environment["name"])
    print(input1)
    return done(outputs={"env_name": environment["name"], "count": count}, tags=["v"+str(count)])

# A destroy function that uses the same inputs as the up function to reverse it
def down(environment, staging_name="", previous_count=0, input1=None):
    print("Destroying minimal package")
    print("Environment: ", environment["name"])
    print("Input1: ", input1)
    return done()