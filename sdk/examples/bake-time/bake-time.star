ocuroot("0.3.0")

load("../common/environments.star", "environments")

def register():
    print("Register bake-time example")
    package(
        name="with_bake_time",
        default_environments=environments,
        release=release,
    )

def release(environments):
    staging_envs = [x for x in environments if x.attributes["type"] == "staging"]
    prod_envs = [x for x in environments if x.attributes["type"] == "prod"]
    
    phases = [
        phase(
            name="staging",
            work=[deploy(up=_deploy_with_bake_time, down=_destroy, environment=env) for env in staging_envs],
        ),
        phase(
            name="bake_time_check",
            work=[call(_check_bake_time)],
        ),
        phase(
            name="production",
            work=[deploy(up=_deploy_production, down=_destroy, environment=env) for env in prod_envs],
        ),
    ]
    return phases

def _check_bake_time(ctx):
    print("Checking bake time for staging deployment")
    return call(_validate_bake_time)

def _validate_bake_time(ctx):
    # Perform bake time validation
    if _has_passed_bake_time(ctx.build.id):
        print("Build passed bake time checks")
        return done()
    else:
        print("Build failed bake time checks")
        return fail("Bake time validation failed")

def _build(ctx):
    print("Building application")
    return done(
        outputs={"artifact": "app-1.0.0.tar.gz"},
    )

def _deploy_with_bake_time(ctx):
    # Check if this build has already been checked and passed
    build_id = ctx.build.id
    if _has_passed_bake_time(build_id):
        print("This build has already passed bake time checks")
        return done(
            outputs={"deploy_id": "staging-deployment-123"},
        )
    
    print("Deploying to staging for bake time")
    return delay(
        fn=_check_telemetry_first,
        delay="5m",  # Start checking after 5 minutes
        inputs={"build_id": build_id, "start_time": ctx.now},
    )

def _check_telemetry_first(ctx):
    build_id = ctx.inputs["build_id"]
    start_time = ctx.inputs["start_time"]
    elapsed_time = ctx.now - start_time
    
    # Check if telemetry is good
    telemetry_ok = _check_telemetry(build_id)
    
    if not telemetry_ok:
        # Fast failure if telemetry is bad
        return fail("Telemetry checks failed after {} minutes".format(elapsed_time / 60))
    
    # Check if we've baked long enough (1 hour)
    if elapsed_time >= 60 * 60:  # 1 hour in seconds
        print("Bake time complete and telemetry is good")
        _mark_build_as_passed(build_id)
        return done(
            outputs={"deploy_id": "staging-deployment-123"},
        )
    else:
        # Continue baking and checking
        return delay(
            fn=_check_telemetry_first,
            delay="5m",  # Check every 5 minutes
            inputs={"build_id": build_id, "start_time": start_time},
        )

def _deploy_production(ctx):
    print("Deploying to production environment")
    return done(
        outputs={"deploy_id": "prod-deployment-123"},
    )

def _destroy(ctx):
    print("Destroying deployment")
    return done()

# Helper functions (simulated)
def _has_passed_bake_time(build_id):
    # In real implementation, this would check against a database or state store
    return False

def _check_telemetry(build_id):
    # In real implementation, this would check metrics, logs, etc.
    return True

def _mark_build_as_passed(build_id):
    # In real implementation, this would update a database or state store
    pass

register()
