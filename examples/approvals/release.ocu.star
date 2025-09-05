ocuroot("0.3.0")

load("../demo_environments.ocu.star", "envs")
staging_envs = [x for x in envs if x.attributes["type"] == "staging"]
prod_envs = [x for x in envs if x.attributes["type"] == "prod"]

def _production_approval(ctx):
    print("Approved!")
    return done()

def _build(ctx):
    print("Building application")
    return next(_build_app, inputs={
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

phase(
    name="build",
    work=[call(_build, name="build")],
)

phase(
    name="staging",
    work=[deploy(
        up=_deploy_environment,
        down=_destroy,
        environment=env,
        inputs={
            "artifact": ref("./task/build#output/artifact"),
        },
    ) for env in staging_envs],
)

phase(
    name="production_approval",
    work=[call(
        _production_approval, 
        name="production_approval",
        inputs={
            # This custom state must be set by the user using `ocuroot state set`
            # Once it is set, and the intent is applied, this flow can continue.
            "approval": ref("./custom/approval"),
        }
    )],
)

phase(
    name="production",
    work=[deploy(
        up=_deploy_environment,
        down=_destroy,
        environment=env,
        inputs={
            "artifact": ref("./task/build#output/artifact"),
        },
    ) for env in prod_envs],
)