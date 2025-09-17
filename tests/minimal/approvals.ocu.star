ocuroot("0.3.0")

load("./tasks.ocu.star", "build", "up", "down")

envs = environments()
staging = [e for e in envs if e.attributes["type"] == "staging"]
prod = [e for e in envs if e.attributes["type"] == "prod"]

phase(
    name="build",
    tasks=[task(build, name="build")],
)

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
        ) for environment in staging
    ],
)

def check(input1, staging_name):
    print("Checking call input")
    return done()

phase(
    name="call input check",
    tasks=[
        task(
            fn=check,
            name="check",
            inputs={
                "input1": ref("./task/build#output/output1"),
                "staging_name": ref("./@/deploy/staging#output/env_name"),
            },
        )
    ]
)

# Require a second approval to proceed
def approve(approval):
    return next(
        fn=_noop,
        inputs={
            "approval": input(
                ref=ref("./custom/approval2"),
            ),
        },
    )

def _noop(approval):
    print("Noop")
    return done()

phase(
    name="prod approval",
    tasks=[task(
        approve, 
        name="prod_approval", 
        inputs={
            "approval": input(
                ref=ref("./custom/approval"),
            ),
        }    
    )],
)

phase(
    name="prod",
    tasks=[
        deploy(
            up=up,
            down=down,
            environment=environment,
            inputs={
                "input1": ref("./task/build#output/output1"),
                "staging_name": ref("./@/deploy/staging#output/env_name"),
                "previous_count": input(
                    ref=ref("./@/deploy/{}#output/count".format(environment.name)),
                    default=0,
                ),
            },
        ) for environment in prod
    ],
)
