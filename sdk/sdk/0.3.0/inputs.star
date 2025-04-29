# TODO: Remove doc from inputs and move it to call/deploy/next.
# This can then provide general instructions for working with the call.
# Also can consider removing the default value here and instead using a default on the function.
# This would make it easier to test and use with the repl.

def input(ref=None, default=None, doc=None):
    """
    input describes an input to a function.

    Args:
        ref: The ref to be used as an input to a call or deployment.
        default: Optional default value to use if the ref is not found. If not provided, the ref is required work will be blocked until it is set.
        doc: Optional documentation string describing how the input can be set and what it is used for.
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
        "doc": doc,
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