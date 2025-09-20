# No ocuroot() call - should use automatic version resolution

load("./tasks.ocu.star", "build", "deploy_test")

# Test without explicit version call
task(build, name="build")

phase(
    name="deploy",
    tasks=[
        task(
            deploy_test,
            name="deploy_test",
            inputs={
                "version_used": ref("./task/build#output/version_used"),
                "test_result": ref("./task/build#output/test_result"),
            },
        )
    ],
)