ocuroot("0.3.0")

def version_to_struct(version):
    prerelease = "0"
    if version.find("-") != -1:
        prerelease = version.split("-")[1]
        version = version.split("-")[0]

    return struct(
        major=int(version.split(".")[0]),
        minor=int(version.split(".")[1]),
        patch=int(version.split(".")[2]),
        prerelease=int(prerelease),
    )

def next_prerelease_version(prerelease, version):
    res = host.shell("cat ./VERSION")
    major_minor = res.stdout.strip()
    MAJOR = major_minor.split(".")[0]
    MINOR = major_minor.split(".")[1]

    mm = "{}.{}.".format(MAJOR, MINOR)
    # First patch sand prerelease for this major/minor version
    if prerelease == "" or not prerelease.startswith(mm):
        return "{}.{}.{}-1".format(MAJOR, MINOR, 0)

    if not version.startswith(mm):
        version = ""

    ps = version_to_struct(prerelease)

    if version != "":
        vs = version_to_struct(version)
        # Next patch for this major/minor version
        if vs.patch == ps.patch:
            return "{}.{}.{}-1".format(MAJOR, MINOR, vs.patch + 1)
    
    # Next prerelease for this major/minor version
    return "{}.{}.{}-{}".format(MAJOR, MINOR, ps.patch, ps.prerelease + 1)