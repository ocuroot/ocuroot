ocuroot("0.3.0")

def register():
    package(
        "my_package",
        default_environments=create_envs(),
        release=_new_release,
    )

def _new_release(environments):
    staging_envs = [x for x in environments if x.attributes["type"] == "staging"]
    prod_envs = [x for x in environments if x.attributes["type"] == "prod"]

    phases = [
        phase(
            name="staging_delay",
            work=[call(_pre_staging)],
        ),
        phase(
            name="staging_deploy",
            work=[deploy(up=_simple_deploy, down=_destroy, environment=env) for env in staging_envs],
        ),
        phase(
            name="staging_test",
            work=[call(_post_staging)],
        ),
        phase(
            name="production_approval",
            work=[call(_pre_prod)],
        ),
        phase(
            name="production_deploy",
            work=[deploy(up=_complex_deploy, down=_destroy, environment=env) for env in prod_envs],
        ),
    ]
    return phases

# Handoff functions
# delay
# approve
# done
# next
# fail? - this would collide with the builtin. Should bear in mind you can always return a failure anyway
# abort - allow you to fail the whole build and revert it

def _pre_staging(ctx):
    return delay(fn=_pre_staging_done, delay="30m")

def _pre_staging_done(ctx):
    return done()

def _post_staging(ctx):
    return done()

def _pre_prod(ctx):
    return approval(
        # A way to check that a user has the ability to approve a step
        #validate=_validate_approval,
        fn=_post_prod,
    )

def _post_prod(ctx):
    return done()

def _simple_deploy(ctx):
    return done()

def create_envs():
    envs = []
    for i in range(1, 3):
        envs.append(environment("staging{}".format(i), {"type": "staging", "channel": str(i)}))
    for i in range(1, 3):
        envs.append(environment("prod{}".format(i), {"type": "prod", "channel": str(i)}))
    return envs

def _build(ctx):
    print("build")
    print(ctx)
    return done()

def _complex_deploy(ctx):
    print("Deploying to environment", ctx.environment.name)
    if True:
        return handoff(_deploy2, annotation="Deploy part 2")
    else:
        return handoff(_deploy3, annotation="Deploy part 3")

def _deploy2(ctx):
    print("Deploy part 2")
    return handoff(_deploy3, annotation="Deploy part 3")

def _deploy3(ctx):
    print("Deploy part 3")

    # TODO: Need a way to abort infinite loops
    if False:
        return handoff(_complex_deploy, annotation="Back to the start")
    else:
        return done()

def _destroy(ctx):
    print("Destroying in environment", ctx.environment.name)

register()
