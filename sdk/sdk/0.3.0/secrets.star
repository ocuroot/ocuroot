def secret(value):
    """
    Register a secret value to be masked in logs
    """

    if type(value) != "string":
        fail("secret value must be a string")

    backend.secrets.secret(json.encode(value))
    return value