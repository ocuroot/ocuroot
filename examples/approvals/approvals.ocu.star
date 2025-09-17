ocuroot("0.3.0")

load("./tasks.ocu.star", "build", "up", "down")

# Run the build first
task(build, name="build")

# Deploy to all staging environments
phase(
    name="staging",
    tasks=[
        deploy(
            up=up,
            down=down,
            environment=environment,
            inputs={
                "input1": ref("./task/build#output/output1"),
                "previous_count": input(
                    ref="./@/deploy/{}#output/count".format(environment.name),
                    default=0,
                ),
            },
        ) for environment in environments() if environment.attributes["type"] == "staging"
    ],
)

# Implement the approval task
def approve(approval):
    return done()

# After deploying to staging, require approvals to deploy to prod
task(
    approve, 
    name="prod_approval", 
    inputs={
        "approval": input(
            ref=ref("./custom/approval"),
        ),
    }    
)

# Deploy to production environments
phase(
    name="prod",
    tasks=[
        deploy(
            up=up,
            down=down,
            environment=environment,
            inputs={
                "input1": ref("./task/build#output/output1"),
                "previous_count": input(
                    ref=ref("./@/deploy/{}#output/count".format(environment.name)),
                    default=0,
                ),
            },
        ) for environment in environments() if environment.attributes["type"] == "prod"
    ],
)
