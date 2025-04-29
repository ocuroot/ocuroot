def secret(value):
    backend.secrets.secret(json.encode(value))
    return {
        "secret": value
    }

def _get_secret(name):
    return json.decode(backend.secrets.get(json.encode(name)))

secrets = struct(
    get=_get_secret,
)