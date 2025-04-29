def _os():
    """
    os returns the OS of the host.
    The value is retrieved using the Go runtime.GOOS constant.

    Returns:
        The OS of the host
    """
    return json.decode(backend.host.os())

def _arch():
    """
    arch returns the architecture of the host.
    The value is retrieved using the Go runtime.GOARCH constant.

    Returns:
        The architecture of the host
    """
    return json.decode(backend.host.arch())

def _env():
    """
    env returns the environment variables of the host as a dictionary.

    Returns:
        The environment variables of the host
    """
    return json.decode(backend.host.env())

def _shell(command, shell="sh", dir=".", env={}, mute=False, continue_on_error=False):
    """
    shell runs a shell command on the host.

    Args:
        command: The command to run
        shell: The shell to use, defaults to "sh"
        dir: The directory to run the command in, relative to the current working directory
        env: The environment variables to set
        mute: Whether to mute the output of the command
        continue_on_error: Whether to continue on error

    Returns:
        A struct containing the combined output, stdout, stderr, and exit code
    """
    resp = backend.host.shell(
        json.encode({
            "cmd": command,
            "shell": shell,
            "dir": dir,
            "env": env,
            "continue_on_error": continue_on_error,
            "mute": mute,
        })
    )

    respDict = json.decode(resp)

    return struct(
        combined_output = respDict["combined_output"],
        stdout = respDict["stdout"],
        stderr = respDict["stderr"],
        exit_code = respDict["exit_code"],
    )

host = struct(
    os = _os,
    arch = _arch,
    env = _env,
    shell = _shell,
)