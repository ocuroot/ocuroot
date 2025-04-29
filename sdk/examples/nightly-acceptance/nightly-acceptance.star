ocuroot("0.3.0")

load("../common/environments.star", "environments")

def register():
    print("Register nightly-acceptance example")
    package(
        name="with_nightly_acceptance",
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
            name="acceptance_test",
            work=[call(_run_nightly_acceptance)],
        ),
        phase(
            name="production_deploy",
            work=[deploy(up=_deploy_environment, down=_destroy, environment=env) for env in prod_envs],
        ),
    ]
    return phases

def _run_nightly_acceptance(ctx):
    # Perform nightly acceptance tests
    print("Running nightly acceptance tests")
    # You can add more complex logic here
    return done()

def _build(ctx):
    print("Building application")
    # Generate a build timestamp to track when it was built
    build_timestamp = ctx.now if hasattr(ctx, "now") else 0

    return done(
        outputs={
            "artifact": "app-1.0.0.tar.gz",
            "build_timestamp": build_timestamp,
        },
    )

def _deploy_environment(ctx):
    env_type = ctx.environment.attributes["type"]

    # For staging, check if this is the latest build
    if env_type == "staging":
        print("Deploying to staging environment")
        return handoff(
            fn=_check_for_nightly_test,
            inputs={
                "stage_deploy_id": "staging-deployment-123",
            },
        )
    elif env_type == "prod":
        print("Deploying to production environment")
        return done(
            outputs={"deploy_id": "prod-deployment-123"},
        )

    return done(
        outputs={"deploy_id": "{}-deployment-123".format(env_type)},
    )

def _check_for_nightly_test(ctx):
    # Check if it's midnight (or appropriate time to run nightly tests)
    is_midnight = _is_midnight_check()

    # Check if this is the latest build
    is_latest_build = _is_latest_build()

    if is_midnight and is_latest_build:
        # Run nightly tests
        print(ctx.inputs)
        return delay(
            fn=_run_nightly_tests,
            delay="5m",  # Simulate running tests for a short time in this example
            inputs={
                "stage_deploy_id": ctx.inputs.stage_deploy_id,
            },
        )
    else:
        # This build is superseded or it's not time for nightly tests yet
        print("Build is not the latest or it's not time for nightly tests")
        return done(
            outputs={"status": "pending_nightly_test"},
        )

def _run_nightly_tests(ctx):
    print("Running nightly acceptance tests")

    # Simulate test results (success or failure)
    tests_passed = _simulate_test_results()

    if tests_passed:
        print("Nightly tests passed, build can proceed to production")
        return done(
            outputs={
                "test_result": "pass",
                "deploy_id": ctx.inputs.stage_deploy_id,
            },
        )
    else:
        # Fail this build and try the next one
        print("Nightly tests failed, failing build")
        return fail("Nightly acceptance tests failed")

def _destroy(ctx):
    print("Destroying deployment")
    return done()

# Helper functions (simulated)
def _is_midnight_check():
    # In a real implementation, check if current time is near midnight or scheduled test time
    return True

def _is_latest_build():
    # In a real implementation, compare against known builds
    return True

def _simulate_test_results():
    # Simulate test results - return True for success, False for failure
    return True

register()
