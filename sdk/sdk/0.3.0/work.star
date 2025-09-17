load("environments.star", "environment_from_json")
load("dowork.star", "check_params")

def next(fn, annotation="", inputs={}):
    """
    next specifies the next function to call within a function chain.

    Args:
        fn: The function to call
        annotation: An optional annotation for the call
        inputs: The inputs to the function, as a dictionary

    Returns:
        A dictionary representing the next work item
    """
    fd = render_function(fn)["function"]

    return {
        "next": {
            "fn": fd,
            "annotation": annotation,
            "inputs": _check_inputs(fn, inputs),
        },
    }

def done(annotation="", outputs={}, tags=[]):
    """
    done marks the end of a function chain.

    Args:
        annotation: An optional annotation for the call
        outputs: The outputs of the chain, as a dictionary
        tags: Optional tags to apply to the release

    Returns:
        A dictionary representing the done work item
    """
    return {
        "done": {
            "outputs": outputs,
            "tags": tags,
        },
    }

def _check_inputs(fn, inputs):
    checked_inputs = {}
    # environment is allowed as an input name here, since
    # you may want to "forward" the value from the previous function
    for key, value in inputs.items():
        if type(value) == "dict" and ("ref" in value or "default" in value or "value" in value):
            checked_inputs[key] = value
        else:
            checked_inputs[key] = {
                "value": value,
            }

    fParams = json.decode(backend.functions.get_args(fn))
    check_params(fn, fParams["args"], fParams["kwargs"], checked_inputs.keys())

    return checked_inputs