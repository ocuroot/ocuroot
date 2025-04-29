def after():
    """
    after is a function that is run after a config script has executed.
    It is used to register packages with the backend.

    It will not be available to the config script.
    """
    if backend.thread.exists("package"):
        package = backend.thread.get("package")
        backend.packages.register(json.encode(package))