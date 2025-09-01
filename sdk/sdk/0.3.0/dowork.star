# do_work is a helper function that calls a function to execute a task or deployment
def do_work(
    f,
    fArgs,
    work_id,
    inputs=None,
):
    if len(fArgs) == 1 and fArgs[0] == "ctx":
        return _do_work_ctx(f, work_id, inputs)

    return _do_work(f, fArgs, work_id, inputs)

def check_params(
    fn,
    fArgs,
    fKWArgs,
    inputs=[],
):
    if len(fArgs) == 1 and fArgs[0] == "ctx":
        return

    fDef = render_function(fn)["function"]
    fName = fDef["name"]
    fPos = fDef["pos"]

    # check that every input is represented in fArgs or fKWArgs
    for k in inputs:
        if k not in fArgs and k not in fKWArgs:
            fail("Input '" + k + "' is not in function parameters for '" + str(fName) + "' at " + str(fPos))

    # check that every fArg is represented in inputs
    for k in fArgs:
        if k not in inputs:
            fail("Function parameter '" + k + "' is not present in inputs and has no default value for '" + str(fName) + "' at " + str(fPos))
    

def _do_work(
    f,
    fArgs,
    work_id,
    inputs=None,
):
    args = {}
    if inputs:
        args = map_from_json(json.decode(inputs))
    return json.encode(f(**args))

def _do_work_ctx(
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

def map_from_json(json):
    vars = {}
    for k, v in json.items():
        # TODO: handle known types using a "$type" field
        # Example below
        if type(v) == "dict" and "$type" in v:
            if v["$type"] == "ref":
                vars[k] = v["ref"]

        vars[k] = v

    return vars
