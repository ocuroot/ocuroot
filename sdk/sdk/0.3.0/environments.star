def environment(name, attributes={}):
    """
    environment defines an environment that can be used for deployment.

    Args:
        name: The name of the environment
        attributes: The attributes of the environment

    Returns:
        A struct containing the name and attributes of the environment
    """
    def _env_to_json():
        return json.encode({
            "name": name,
            "attributes": attributes,
        })

    def _env_to_dict():
        return {
            "name": name,
            "attributes": attributes,
        }

    return struct(
        name=name,
        attributes=attributes,
        json=_env_to_json,
        dict=_env_to_dict,
    )

def environment_from_json(envJSON):
    env = json.decode(envJSON)
    return environment(
        name=env["name"],
        attributes=env["attributes"],
    )

def environment_from_dict(envDict):
    return environment(
        name=envDict["name"],
        attributes=envDict["attributes"],
    )

def environments():
    """
    environments returns a list of all registered environments.

    Returns:
        A list of all registered environments
    """
    envs = backend.environments.all()
    e = json.decode(envs)
    if e == None:
        return []
    return [environment_from_dict(env) for env in e]

def register_environment(env):
    """
    register_environment registers an environment that can be used for deployment.

    Args:
        env: The environment to register
    """
    backend.environments.register(env.json())