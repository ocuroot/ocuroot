ocuroot("0.3.0")

load("./tasks.ocu.star", "build", "up", "down")

envs = environments()
staging = [e for e in envs if e.attributes["type"] == "staging"]
prod = [e for e in envs if e.attributes["type"] == "prod"]

task(build, name="build")

phase(
    name="staging",
    tasks=[
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
