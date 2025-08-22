def after():
    """
    after is a function that is run after a config script has executed.
    It is used to register packages with the backend.

    It will not be available to the config script.
    """
    if backend.thread.exists("package"):
        package = backend.thread.get("package")

        # Create phases for remaining registered work        
        registered_work = backend.thread.get("work", default=[])
        for w in registered_work:
            package["phases"].append({
                "name": "",
                "work": [w],
            })

        backend.packages.register(json.encode(package))