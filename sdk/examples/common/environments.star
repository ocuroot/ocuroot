ocuroot("0.3.0")

load("prod_count.star", "prod_count")

def register_envs():
    environments = []
    environments.append(environment("staging1", {"type": "staging"}))
    for i in range(0, prod_count):
        environments.append(environment("prod{}".format(i), {"type": "prod", "canary": str(i==0)}))
    return environments

environments = register_envs()
