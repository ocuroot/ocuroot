ocuroot("0.3.0")

load("./tasks.ocu.star", "build", "do_release")

MAJOR = "0"
MINOR = "1"

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

def prerelease(prev_prerelease, prev_version):
    prerelease = next_prerelease_version(prev_prerelease, prev_version)
    return done(
        outputs={
            "prerelease": prerelease,
        },
        tags=[prerelease],
    )

phase(
    name="version",
    tasks=[task(
        prerelease, 
        name="prerelease", 
        inputs={
            "prev_prerelease": input(ref="./@/task/version#output/prerelease", default=""),
            "prev_version": input(ref="./@/task/finalize#output/version", default=""),
        },
    )],
)

phase(
    name="build",
    tasks=[task(
        build, 
        name="build", 
        inputs={
            "prerelease": input(ref="./@/task/prerelease#output/prerelease"),
        },
    )],
)

promotion_ref = ref("./custom/promote")

def promote():
    print("Promoting")
    return done()

phase(
    name="promote",
    tasks=[task(
        promote, 
        name="promote", 
        inputs={
            "approval": input(
                ref=promotion_ref,
                doc="""Manually promote this release by running 
        ocuroot state set \"{ref}\" 1
        ocuroot state apply \"{ref}\"""".format(ref=promotion_ref["ref"].replace("@", "+")),
            ),

        }    
    )],
)

def release(prerelease):
    version = prerelease.split("-")[0]

    do_release(version)

    return done(
        outputs={
            "version": version,
        },
        tags=[version],
    )

phase(
    name="release",
    tasks=[
        task(
            release,
            name="release",
            inputs={
                "prerelease": input(ref="./@/task/prerelease#output/prerelease"),
            },
        ),
    ],
)

def finalize(prerelease):
    version = prerelease.split("-")[0]
    return done(
        outputs={
            "version": version,
        },
    )

phase(
    name="finalize",
    tasks=[
        task(
            finalize,
            name="finalize",
            inputs={
                "version": input(ref="./@/task/version#output/version"),
            },
        ),
    ],
)