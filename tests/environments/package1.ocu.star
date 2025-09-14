ocuroot("0.3.0")

def up(environment):
    print("package1 up")
    write("./.deploys/{}/package1.txt".format(environment["name"]), str(environment))
    return done()

def down(environment):
    print("package1 down")
    content = read("./.deploys/{}/package1.txt".format(environment["name"]))
    if content != str(environment):
        return fail("content does not match")
    shell("rm -f ./.deploys/{}/package1.txt".format(environment["name"]))
    return done()

phase(
    "deploy",
    tasks=[
        deploy(
            up=up,
            down=down,
            environment=environment,
        ) for environment in environments()
    ]
)