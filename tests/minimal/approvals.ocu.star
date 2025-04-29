ocuroot("0.3.0")

load("./tasks.ocu.star", "build", "up", "down")

envs = environments()
staging = [e for e in envs if e.attributes["type"] == "staging"]
prod = [e for e in envs if e.attributes["type"] == "prod"]

phase(
    name="build",
    work=[call(build, name="build")],
)

phase(
    name="staging",
    work=[
        deploy(
            up=up,
            down=down,
            environment=environment,
            inputs={
                "input1": ref("./call/build#output/output1"),
                "previous_count": input(
                    ref="./@/deploy/{}#output/count".format(environment.name),
                    default=0,
                ),
            },
        ) for environment in staging
    ],
)

def check(ctx):
    print("Checking call input")
    print(ctx)
    return done()

phase(
    name="call input check",
    work=[
        call(
            fn=check,
            name="check",
            inputs={
                "input1": ref("./call/build#output/output1"),
                "staging_name": ref("./@/deploy/staging#output/env_name"),
            },
        )
    ]
)

# Require a second approval to proceed
def approve(ctx):
    return next(
        fn=_noop,
        inputs={
            "approval": input(
                ref=ref("./custom/approval2"),
            ),
        },
    )

def _noop(ctx):
    print("Noop")
    return done()

phase(
    name="prod approval",
    work=[call(
        approve, 
        name="prod_approval", 
        inputs={
            "approval": input(
                ref=ref("./custom/approval"),
                doc="""Manually approve this step by running 
        ocuroot state set \"{approval_ref}\" 1
        ocuroot state apply \"{approval_ref}\"""".format(approval_ref=ref("./custom/approval")["ref"].replace("@", "+")),
            ),
        }    
    )],
)

phase(
    name="prod",
    work=[
        deploy(
            up=up,
            down=down,
            environment=environment,
            inputs={
                "input1": ref("./call/build#output/output1"),
                "staging_name": ref("./@/deploy/staging#output/env_name"),
                "previous_count": input(
                    ref=ref("./@/deploy/{}#output/count".format(environment.name)),
                    default=0,
                ),
            },
        ) for environment in prod
    ],
)
