load("environments.star", "environment_from_json")

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
    # TODO: This might not be necessary
    # add_func(fd)

    return {
        "next": {
            "fn": fd,
            "annotation": annotation,
            "inputs": _check_inputs(inputs),
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

def do_work(
    f,
    work_id,
    inputs=None,
):
    args = {}
    if work_id:
        args["work_id"] = json.decode(work_id)
    if inputs:
        args["inputs"] = vars_from_json(json.decode(inputs))

    ctx = struct(**args)
    return json.encode(f(ctx))

def vars_from_json(json):
    vars = {}
    for k, v in json.items():
        # TODO: handle known types using a "$type" field
        # Example below
        if type(v) == "dict" and "$type" in v:
            if v["$type"] == "ref":
                vars[k] = v["ref"]

        vars[k] = v

    return struct(**vars)

def _check_inputs(inputs):
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
    return checked_inputs