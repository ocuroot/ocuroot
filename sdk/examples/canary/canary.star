ocuroot("0.3.0")

load("../common/environments.star", "environments")

def register():
    package(
        name="with_canary",
        default_environments=environments,
        release=release,
   )

def release(environments):
    staging_envs = [x for x in environments if x.attributes["type"] == "staging"]
    prod_envs = [x for x in environments if x.attributes["type"] == "prod"]
    
    phases = [
        phase(
            name="staging_deploy",
            work=[deploy(up=_deploy_no_canary, down=_destroy, environment=env) for env in staging_envs],
        ),
        phase(
            name="prod_canary_deploy",
            work=[deploy(up=_deployed_this_build_before, down=_destroy, environment=env) 
                  for env in prod_envs if env.attributes.get("canary", "False") == "True"],
        ),
        phase(
            name="prod_deploy",
            work=[deploy(up=_deploy_no_canary, down=_destroy, environment=env) 
                  for env in prod_envs if env.attributes.get("canary", "False") == "False"],
        ),
    ]
    return phases

def _build(ctx):
    print("build")
    print(ctx)
    return handoff(_build2, inputs={
        "input1": 5.5,
    })

def _build2(ctx):
    print("build2")
    print(ctx)
    return done(
        outputs={"output1": 5.5},
    )

def _deployed_this_build_before(ctx):
    if False:
        return handoff(_deploy_no_canary, annotation="yes")
    else:
        return handoff(_deploy_canary, annotation="no")

def _deploy_no_canary(ctx):
    print("exec: ./deploy.sh")
    return done(
        outputs={"deploy_id": "id"},
    )

def _deploy_canary(ctx):
    print("exec: ./deploy_canary.sh")
    return delay(
        fn=_check_and_promote_canary,
        delay="1h",
        inputs={"canary_id": "id"},
    )

def _check_and_promote_canary(ctx):
    print("exec: ./check_canary.sh")
    if True:
        # Revert the build
        return fail("The canary failed")

    # Promote the canary
    print("exec: ./promote_canary.sh")
    return done(
        outputs={"canary_id": "id"},
    )

def _destroy(ctx):
    print("exec: ./destroy.sh")
    return done()

register()
