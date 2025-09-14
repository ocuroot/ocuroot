load("dowork.star", "check_params")

def phase(name, work=[], tasks=[]):
    """
    phase defines a single phase within a release.
    A release function should return a list of phases.
    Phases are executed in the order they are returned.

    Tasks may be defined using the following functions:
    - deploy: A deployment to a specific environment
    - task: A standalone task, for example a build or test

    Args:
        name: The name of the phase
        tasks: A list of tasks to perform in this phase

    Returns:
        A dictionary representing the phase
    """
    if len(tasks) == 0:
        tasks = work

    package = backend.thread.get("package", default={
        "phases": [],
        "functions": {},
    })

    # Create new phases for any tasks that are not in this phase
    registered_tasks = backend.thread.get("tasks", default=[])
    for t in registered_tasks:
        match = False
        for r in tasks:
            if r["task_id"] == t["task_id"]:
                match = True
                break
        if not match:
            package["phases"].append({
                "name": "",
                "tasks": [t],
            })
    backend.thread.set("tasks", [])

    # Add this phase to the list stored on the thread
    package["phases"].append({
        "name": name,
        "tasks": tasks,
    })
    backend.thread.set("package", package)

    return {
        "phase": {
            "name": name,
            "tasks": tasks,
        },
    }

_default_up = lambda ctx, result: None
_default_down = lambda ctx, result: None

def deploy(environment=None, up=_default_up, down=_default_down, inputs={}):
    """
    deploy defines a deployment to a specific environment as a task.

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

    checked_inputs = _check_inputs(up, inputs, environment)
    _check_inputs(down, inputs, environment) # Check without keeping the results

    # Make the environment an implicit input to this deployment
    checked_inputs["environment"] = {
        "ref": "@/environment/{}".format(environment.name),
    }

    r_up = render_function(up, require_top_level=True)["function"]
    r_down = render_function(down, require_top_level=True)["function"]
    
    _add_func(r_up)
    _add_func(r_down)

    task = {
        "task_id": backend.ulid(),
        "deploy": {
            "environment": environment.name,
            "up": r_up,
            "down": r_down,
            "inputs": checked_inputs,
        },
    }

    # Add this task to the list stored on the thread
    task_items = backend.thread.get("tasks", default=[])
    task_items.append(task)
    backend.thread.set("tasks", task_items)

    return task


def call(fn, name, annotation="", inputs={}):
    return task(fn, name, annotation, inputs)

def task(fn, name, annotation="", inputs={}):
    """
    task defines a standalone task that is part of a release.

    The function provided to fn must return with either of the following functions:
    - done
    - next
    These functions may also error out or exit using the fail function.

    Args:
        fn: The function implementing this task
        name: The name of the task, which must be unique within the release
        annotation: An optional annotation for the task
        inputs: The inputs to the function, as a dictionary

    Returns:
        A dictionary representing the task
    """

    checked_inputs = _check_inputs(fn, inputs)
    fn = render_function(fn)["function"]
    _add_func(fn)

    task = {
        "task_id": backend.ulid(),
        "task": {
            "fn": fn,
            "name": name,
            "annotation": annotation,
            "inputs": checked_inputs,
        },
    }

    # Add this task to the list stored on the thread
    tasks = backend.thread.get("tasks", default=[])
    tasks.append(task)
    backend.thread.set("tasks", tasks)

    return task

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

def _check_inputs(fn, inputs, environment=None):
    checked_inputs = {}
    for key, value in inputs.items():
        if key == "environment" and environment:
            fail("environment is a reserved input for deployments")
        if type(value) == "dict" and ("ref" in value or "default" in value or "value" in value):
            checked_inputs[key] = value
        else:
            checked_inputs[key] = {
                "value": value,
            }
    if environment:
        checked_inputs["environment"] = {"ref": "@/environment/{}".format(environment.name)}

    fParams = json.decode(backend.functions.get_args(fn))
    check_params(fn, fParams["args"], fParams["kwargs"], checked_inputs.keys())

    return checked_inputs