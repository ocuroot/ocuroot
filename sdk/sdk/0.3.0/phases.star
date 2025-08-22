def phase(name, work=[]):
    """
    phase defines a single phase within a release.
    A release function should return a list of phases.
    Phases are executed in the order they are returned.

    Work items may be defined using the following functions:
    - deploy: A deployment to a specific environment
    - call: A call to a function, for builds or tests

    Args:
        name: The name of the phase
        work: A list of work items to perform in this phase

    Returns:
        A dictionary representing the phase
    """
    
    package = backend.thread.get("package", default={
        "phases": [],
        "functions": {},
    })

    # Create new phases for any work items that are not in this phase
    registered_work = backend.thread.get("work", default=[])
    for w in registered_work:
        match = False
        for r in work:
            if r["work_id"] == w["work_id"]:
                match = True
                break
        if not match:
            package["phases"].append({
                "name": "",
                "work": [w],
            })
    backend.thread.set("work", [])

    # Add this phase to the list stored on the thread
    package["phases"].append({
        "name": name,
        "work": work,
    })
    backend.thread.set("package", package)

    return {
        "phase": {
            "name": name,
            "work": work,
        },
    }

_default_up = lambda ctx, result: None
_default_down = lambda ctx, result: None

def deploy(environment=None, up=_default_up, down=_default_down, inputs={}):
    """
    deploy defines a deployment to a specific environment as a work item in a phase.

    The functions provided to up and down must return with either of the following functions:
    - done
    - next
    These functions may also error out or exit using the fail function.

    Args:
        environment: The environment to deploy to
        up: The function to run when deploying the resource
        down: The function to run when destroying the resource
        inputs: The inputs to the function, which must be declared using the input() function

    Returns:
        A dictionary representing the deploy action
    """

    checked_inputs = _check_inputs(inputs)
    # Make the environment an implicit input to this deployment
    checked_inputs["environment"] = {
        "ref": "@/environment/{}".format(environment.name),
    }

    r_up = render_function(up, require_top_level=True)["function"]
    r_down = render_function(down, require_top_level=True)["function"]
    
    _add_func(r_up)
    _add_func(r_down)

    work = {
        "work_id": backend.ulid(),
        "deploy": {
            "environment": environment.name,
            "up": r_up,
            "down": r_down,
            "inputs": checked_inputs,
        },
    }

    # Add this work item to the list stored on the thread
    work_items = backend.thread.get("work", default=[])
    work_items.append(work)
    backend.thread.set("work", work_items)

    return work


def call(fn, name, annotation="", inputs={}):
    """
    call requests that a function be called.

    The function provided to fn must return with either of the following functions:
    - done
    - next
    These functions may also error out or exit using the fail function.

    Args:
        fn: The function to call
        name: The name of the call, which must be unique within the release
        annotation: An optional annotation for the call
        inputs: The inputs to the function, as a dictionary

    Returns:
        A dictionary representing the call work item
    """

    checked_inputs = _check_inputs(inputs)
    fn = render_function(fn)["function"]
    _add_func(fn)

    work = {
        "work_id": backend.ulid(),
        "call": {
            "fn": fn,
            "name": name,
            "annotation": annotation,
            "inputs": checked_inputs,
        },
    }

    # Add this work item to the list stored on the thread
    work_items = backend.thread.get("work", default=[])
    work_items.append(work)
    backend.thread.set("work", work_items)

    return work

def _env_or_name(env):
    if type(env) == str:
        return env
    return env.name

def _add_func(func):
    if func == None:
        return
    
    package = backend.thread.get("package", default={
        "phases": [],
        "functions": {},
    })
    functions = package["functions"]
    if not func in functions:
        fd = {
            "function": func,
        }
        functions[func["name"]+"/"+func["pos"]] = fd

    package["functions"] = functions
    backend.thread.set("package", package)

def _check_inputs(inputs):
    checked_inputs = {}
    for key, value in inputs.items():
        if key == "environment":
            fail("environment is a reserved input for deployments")
        if type(value) == "dict" and ("ref" in value or "default" in value or "value" in value):
            checked_inputs[key] = value
        else:
            checked_inputs[key] = {
                "value": value,
            }
    return checked_inputs