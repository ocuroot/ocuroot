def input(ref=None, default=None):
    """
    input describes an input to a function.

    Args:
        ref: The ref to be used as an input to a call or deployment.
        default: Optional default value to use if the ref is not found. If not provided, the ref is required work will be blocked until it is set.
    """
    if ref == None and default == None:
        fail("input must have either ref or default")
    
    if ref != None:
        if type(ref) == "dict":
            ref = ref["ref"]
        ref = json.decode(backend.refs.absolute(json.encode(ref)))
    
    return {
        "ref": ref,
        "default": default,
    }

def ref(ref):
    """
    ref describes a ref to be used as an input to a call or deployment.

    Args:
        ref: The ref. This may be absolute or relative. Output will be absolute.
    """
    ref = json.decode(backend.refs.absolute(json.encode(ref)))
    
    return {
        "ref": ref,
    }