
def directories_in_go_build(pkg):
    """
    Returns a list of directories containing source files in the Go build
    for a package, relative to the module root.

    The module root is obtained based on the go.mod closes to the
    current working directory.

    Source files outside the module are ignored.

    Args:
        pkg: The go package to operate on.
    """

    # Get the module path to ensure we're always in the root of the Go module
    baseDir = shell("go list -f '{{.Module.Dir}}' .",mute=True).stdout.strip()

    # Template to output all files in the Go build (including embeds), one per line
    template = """{{range .GoFiles}}{{$.Dir}}/{{.}}{{"\\n"}}{{end}}{{range .EmbedFiles}}{{$.Dir}}/{{.}}{{"\\n"}}{{end}}"""

    # Run go list with template on given package and break into lines
    list_result=shell("go list -deps -f '{}' {}".format(template, pkg), 
        dir=baseDir,
        mute=True,
    ).stdout.strip()
    files=list_result.split("\n")

    # Reduce to a list of directories containing source files,
    # relative to the module root
    out = []
    for file in files:
        if not file.startswith(baseDir):
            continue

        o = file
        o = o.replace(baseDir + "/", "")
        o = "/".join(o.split("/")[0:-1])
        if not o in out:
            out.append(o)
    return out

# Example to test in the REPL
# directories_in_go_build("./cmd/ocuroot")