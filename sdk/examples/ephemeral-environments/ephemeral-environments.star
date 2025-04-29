ocuroot("0.3.0")

load("../common/environments.star", "environments")

def register():
    print("Register ephemeral-environments example")
    package(
        name="with_ephemeral_environments",
        default_environments=environments,
        release=release,
    )

def release(environments):
    # We handle PR environments separately from standard environments
    standard_envs = [x for x in environments if not x.attributes.get("is_pr", False)]
    pr_envs = [x for x in environments if x.attributes.get("is_pr", False)]

    # For standard flow (non-PR environments)
    standard_phases = [
        phase(
            name="staging_deploy",
            work=[deploy(up=_deploy_standard_environment, down=_destroy, environment=env) 
                  for env in standard_envs if env.attributes["type"] == "staging"],
        ),
        phase(
            name="production_deploy",
            work=[deploy(up=_deploy_standard_environment, down=_destroy, environment=env) 
                  for env in standard_envs if env.attributes["type"] == "prod"],
        ),
    ]

    # For PR environments
    pr_phases = [
        phase(
            name="pr_{}".format(pr_env.attributes.get("pr_number", "unknown")),
            work=[deploy(up=_deploy_pr_environment, down=_destroy, environment=pr_env)]
        ) for pr_env in pr_envs
    ]

    # Combine phases
    return standard_phases + pr_phases

def _build(ctx):
    print("Building application")

    # Get PR information if available
    pr_number = ctx.metadata.get("pr_number")

    if pr_number:
        print("Building for PR #{}".format(pr_number))

    return done(
        outputs={"artifact": "app-1.0.0.tar.gz"},
    )

def _deploy_pr_environment(ctx):
    pr_number = ctx.environment.attributes.get("pr_number", "unknown")
    print("Deploying PR #{} environment".format(pr_number))

    # Set maximum lifetime for PR environment (destroy after 7 days)
    expiration_time = ctx.now + (7 * 24 * 60 * 60)  # 7 days in seconds

    return done(
        outputs={
            "deploy_id": "pr-{}-deployment".format(pr_number),
            "expiration_time": expiration_time,
        },
    )

def _deploy_standard_environment(ctx):
    env_type = ctx.environment.attributes["type"]
    print("Deploying to {} environment".format(env_type))
    return done(
        outputs={"deploy_id": "{}-deployment-123".format(env_type)},
    )

def _destroy(ctx):
    # Check if this is a PR environment being destroyed due to merge
    if ctx.environment.attributes.get("is_pr", False) and ctx.metadata.get("pr_merged", False):
        pr_number = ctx.environment.attributes.get("pr_number", "unknown")
        print("PR #{} was merged, destroying ephemeral environment".format(pr_number))
    elif ctx.environment.attributes.get("is_pr", False) and ctx.metadata.get("pr_closed", False):
        pr_number = ctx.environment.attributes.get("pr_number", "unknown")
        print("PR #{} was closed or abandoned, destroying ephemeral environment".format(pr_number))
    else:
        print("Destroying deployment")

    return done()

register()
