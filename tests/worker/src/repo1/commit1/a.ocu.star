ocuroot("0.3.0")

def up(environment, version):
    worker_id = env()["WORKER_ID"]
    deploy_dir = "{}/a/{}/{}".format(env()["DEPLOY_DIR"],environment["name"], version)
    message = read("a.txt")

    # Create the dir if it doesn't exist
    shell("mkdir -p {}".format(deploy_dir))
    
    marker_file = "{}/{}".format(deploy_dir, worker_id)
    print("Writing marker to {}".format(marker_file))    
    shell('echo "$MESSAGE" > $PATH', env={
        "MESSAGE": message,
        "PATH": marker_file,
    })

    return done(
        outputs={
            "message": message,
            "version": version+1,
        },
        watch=["a.txt"],
    )

def down(environment, version):
    deploy_dir = "{}/a/{}".format(env()["DEPLOY_DIR"],environment["name"])

    shell("rm -rf {}".format(deploy_dir))
    return done()

phase(
    name="deploy",
    tasks=[
        deploy(
            up=up,
            down=down,
            environment=environment,
            inputs={
                "version": input(ref="./@/deploy/{}#output/version".format(environment.name), default=0),
            },
        ) for environment in environments()
    ],
)