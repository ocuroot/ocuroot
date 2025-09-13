def after():
    """
    after is a function that is run after a config script has executed.
    It is used to register packages with the backend.

    It will not be available to the config script.
    """
    if backend.thread.exists("package"):
        package = backend.thread.get("package")

        # Create phases for remaining registered tasks    
        registered_tasks = backend.thread.get("tasks", default=[])
        for w in registered_tasks:
            package["phases"].append({
                "name": "",
                "tasks": [w],
            })

        backend.packages.register(json.encode(package))