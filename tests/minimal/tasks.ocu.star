ocuroot("0.3.0")

def build():
    print("Building minimal package")

    # Read source file and write to output file
    src = read("src.txt")
    expected_src = "Content here"
    if src != expected_src:
        fail("Source file content does not match expected value. Got: " + src)
    write(".build/output.txt", src)
    dir_content = read_dir(".build")
    if not is_dir(".build"):
        fail(".build directory not found")
    if len(dir_content) != 1:
        fail("Expected 1 file in .build directory, got: " + str(len(dir_content)))
    if "output.txt" not in dir_content:
        fail("Output file not found in .build directory")
    if read(".build/output.txt") != expected_src:
        fail("Output file content does not match expected value. Got: " + read(".build/output.txt"))

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

def up(environment, input1, previous_count=0, staging_name=""):
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

    return next(up_stage2, inputs=outputs)

def up_stage2(count, environment, input1):
    print("Continuing deploy to ", environment["name"])
    print(input1)
    return done(outputs={"env_name": environment["name"], "count": count}, tags=["v"+str(count)])

def down(environment, staging_name="", previous_count=0, input1=None):
    print("Destroying minimal package")
    print("Environment: ", environment["name"])
    print("Input1: ", input1)
    return done()