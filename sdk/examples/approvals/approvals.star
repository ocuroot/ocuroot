ocuroot("0.3.0")

load("../common/environments.star", "environments")

def register():
    print("Register approvals example")
    package(
        name="with_approvals",
        default_environments=environments,
        release=release,
    )

def release(environments):
    staging_envs = [x for x in environments if x.attributes["type"] == "staging"]
    prod_envs = [x for x in environments if x.attributes["type"] == "prod"]

    # Define phases with approval requirement for production
    phases = [
        phase(
            name="staging",
            work=[deploy(up=_deploy_environment, down=_destroy, environment=env) for env in staging_envs],
        ),
        phase(
            name="production_approval",
            work=[call(_production_approval)],
        ),
        phase(
            name="production",
            work=[deploy(up=_deploy_environment, down=_destroy, environment=env) for env in prod_envs],
        ),
    ]
    return phases

def _production_approval(ctx):
    print("Requesting approval for production phase")
    return approval(
        fn=_approve_production,
    )

def _approve_production(ctx):
    print("Production phase approved")
    return done()

def _build(ctx):
    print("Building application")
    return handoff(_build_app, inputs={
        "version": "1.0.0",
    })

def _build_app(ctx):
    print("Building application with version: {}".format(ctx.inputs["version"]))
    return done(
        outputs={"artifact": "app-{}.tar.gz".format(ctx.inputs["version"])},
    )

def _deploy_environment(ctx):
    # Function to handle actual deployment after potential approval
    env_type = ctx.environment.attributes["type"]
    print("ctx:", ctx)

    print("Deploying to {} environment".format(env_type))
    return done(
        outputs={"deploy_id": "{}-deployment-123".format(env_type)},
    )

def _destroy(ctx):
    print("Destroying deployment")
    return done()

register()
