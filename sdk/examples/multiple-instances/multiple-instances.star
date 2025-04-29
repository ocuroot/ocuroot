ocuroot("0.3.0")

load("../common/environments.star", "environments")

def register():
    print("Register multiple-instances example")
    package(
        name="with_multiple_instances",
        default_environments=environments,
        release=release,
    )

def release(environments):
    staging_envs = [x for x in environments if x.attributes["type"] == "staging"]
    prod_envs = [x for x in environments if x.attributes["type"] == "prod"]

    phases = [
        phase(
            name="staging_deploy",
            work=[deploy(up=_deploy_environment, down=_destroy, environment=env) for env in staging_envs],
        ),
        phase(
            name="production_deploy",
            work=[deploy(up=_deploy_environment, down=_destroy, environment=env) for env in prod_envs],
        ),
    ]
    return phases

def _build(ctx):
    print("Building application")

    # Generate a unique ID for this instance
    # Use a simple timestamp as a unique identifier
    import_time = 1714167600  # Hardcoded example timestamp
    instance_id = "instance-{}".format(import_time)

    return done(
        outputs={
            "artifact": "app-1.0.0.tar.gz",
            "instance_id": instance_id,
        },
    )

def _deploy_environment(ctx):
    env_type = ctx.environment.attributes["type"]
    # Handle potential missing outputs
    instance_id = "unknown"
    if hasattr(ctx, "build") and hasattr(ctx.build, "outputs"):
        if "instance_id" in ctx.build.outputs:
            instance_id = ctx.build.outputs["instance_id"]

    print("Deploying instance {} to {} environment".format(instance_id, env_type))

    # Deploy as a new instance that can run in parallel with existing instances
    return done(
        outputs={
            "deploy_id": "{}-{}-deployment".format(env_type, instance_id),
            "instance_id": instance_id,
            "url": "https://{}.{}.example.com".format(instance_id, env_type),
        },
    )

def _destroy(ctx):
    instance_id = ctx.build.outputs.get("instance_id", "unknown")
    print("Destroying instance {} deployment".format(instance_id))
    return done()

register()
