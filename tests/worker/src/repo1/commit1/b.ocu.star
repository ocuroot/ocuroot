ocuroot("0.3.0")

def up(environment, message):
    worker_id = env()["WORKER_ID"]
    deploy_dir = "{}/b/{}".format(env()["DEPLOY_DIR"],environment["name"])

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
        },
        watch=["b.ocu.star"],
    )

def down(environment, message):
    deploy_dir = "{}/b/{}".format(env()["DEPLOY_DIR"],environment["name"])

    shell("rm -rf {}".format(deploy_dir))
    return done()

phase(
    name="staging",
    tasks=[
        deploy(
            up=up,
            down=down,
            environment=environment,
            inputs={
                "message": input(
                    ref="./-/a.ocu.star/@/deploy/{}#output/message".format(environment.name),
                ),
            },
        ) for environment in environments()
    ],
)