_default_release = lambda environments, result: None

def unwrap(d, key):
    if key not in d:
        fail("expected key {} in {}".format(key, d))
    return d[key]

def outputs(**kwargs):
    result = {
        "outputs": {}
    }
    for k, v in kwargs.items():
        result["outputs"][k] = v
    return result