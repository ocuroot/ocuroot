ocuroot("0.3.0")

load("../common/environments.star", "environments")

def register():
    print("Register deployment-freezes example")
    package(
        name="with_deployment_freezes",
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
            work=[deploy(up=_deploy_with_freeze_check, down=_destroy, environment=env) for env in prod_envs],
        ),
    ]
    return phases

def _build(ctx):
    print("Building application")
    return done(
        outputs={"artifact": "app-1.0.0.tar.gz"},
    )

def _deploy_with_freeze_check(ctx):
    # Check if a deployment freeze is in place
    freeze_active = _check_freeze_status()

    if freeze_active:
        # If freeze is active, wait and retry later
        print("Deployment freeze is active, waiting...")
        return delay(
            fn=_deploy_with_freeze_check,
            delay="1h",  # Check again in an hour
            inputs={},
        )

    # No freeze, proceed with deployment
    print("No freeze active, deploying to production")
    return done(
        outputs={"deploy_id": "prod-deployment-123"},
    )

def _deploy_environment(ctx):
    env_type = ctx.environment.attributes["type"]
    print("Deploying to {} environment".format(env_type))
    return done(
        outputs={"deploy_id": "{}-deployment-123".format(env_type)},
    )

def _destroy(ctx):
    print("Destroying deployment")
    return done()

# Helper function to check freeze status (simulated)
def _check_freeze_status():
    # In a real implementation, this would query a database or API
    # to determine if a freeze is currently active
    return False  # No freeze active by default

register()
