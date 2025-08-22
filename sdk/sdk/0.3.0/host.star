def os():
    """
    os returns the OS of the host.
    The value is retrieved using the Go runtime.GOOS constant.

    Returns:
        The OS of the host
    """
    return json.decode(backend.host.os())

def arch():
    """
    arch returns the architecture of the host.
    The value is retrieved using the Go runtime.GOARCH constant.

    Returns:
        The architecture of the host
    """
    return json.decode(backend.host.arch())

def env():
    """
    env returns the environment variables of the host as a dictionary.

    Returns:
        The environment variables of the host
    """
    return json.decode(backend.host.env())

def shell(command, shell="sh", dir=".", env={}, mute=False, continue_on_error=False):
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

def pwd():
    """
    pwd returns the current working directory of the host.

    Returns:
        The current working directory of the host
    """
    return json.decode(backend.host.working_dir())

def read(path):
    """
    read reads the contents of a file on the host.

    Args:
        path: The path to the file to read

    Returns:
        The contents of the file
    """
    return json.decode(backend.host.read_file(json.encode(path)))

def write(path, content):
    """
    write writes the contents of a file on the host.

    Args:
        path: The path to the file to write
        content: The contents of the file to write

    Returns:
        None
    """
    return json.decode(backend.host.write_file(
        json.encode({
            "path": path,
            "content": content,
        })
    ))

def read_dir(path):
    """
    read_dir reads the contents of a directory on the host.

    Args:
        path: The path to the directory to read

    Returns:
        The contents of the directory
    """
    return json.decode(backend.host.read_dir(json.encode(path)))

def is_dir(path):
    """
    is_dir checks if a path is a directory on the host.

    Args:
        path: The path to check

    Returns:
        True if the path is a directory, False otherwise
    """
    return json.decode(backend.host.is_dir(json.encode(path)))    

host = struct(
    os = os,
    arch = arch,
    env = env,
    shell = shell,
    pwd = pwd,
    read_file = read,
    write_file = write,
    read_dir = read_dir,
    is_dir = is_dir,
)