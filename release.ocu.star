ocuroot("0.3.0")

load("./versions.ocu.star", "next_prerelease_version")

def unit_test(ctx):
    print("Running unit tests")
    host.shell("go test ./...")
    return done()

phase(
    name="unit",
    work=[call(unit_test, name="unit_test")],
)

def endtoend(ctx):
    print("Running end-to-end tests")
    host.shell("make e2e")
    return done()

phase(
    name="endtoend",
    work=[call(endtoend, name="endtoend")],
)

def increment_version(ctx):
    prerelease = next_prerelease_version(ctx.inputs.prev_prerelease, ctx.inputs.prev_version)
    return done(
        outputs={
            "prerelease": prerelease,
        },
        tags=[prerelease],
    )

phase(
    name="version",
    work=[call(
        increment_version, 
        name="increment_version", 
        inputs={
            "prev_prerelease": input(ref="./@/call/increment_version#output/prerelease", default="0.3.4-1"),
            "prev_version": input(ref="./@/call/release#output/version", default="0.3.4"),
        },
    )],
)

oses = ["linux", "darwin", "windows"]
arches = ["amd64", "arm64"]

def build(ctx):
    os = ctx.inputs.os
    arch = ctx.inputs.arch
    version = ctx.inputs.version

    # Build the binary for this platform
    output = "./.build/{os}-{arch}/ocuroot".format(os=os, arch=arch)
    host.shell("GOOS={os} GOARCH={arch} go build -o {output} ./cmd/ocuroot".format(os=os, arch=arch, output=output))
    
    # Upload to R2
    host.shell(
        "rclone copy {output} ocuroot_binaries:client-binaries/ocuroot/{version}/{os}-{arch}".format(
            os=os, 
            arch=arch, 
            output=output, 
            version=version,    
        )
    )

    # Output the URL for future use
    url = "https://downloads.ocuroot.com/ocuroot/{version}/{os}-{arch}/ocuroot".format(os=os, arch=arch, version=version)
    return done(outputs={"download_url": url})

phase(
    name="build",
    work=[
        call(
            build,
            name="build_{os}_{arch}".format(os=os, arch=arch),
            inputs={
                "os": os,
                "arch": arch,
                "version": input(ref="./call/increment_version#output/prerelease"),
            },
        )
        for os in oses
        for arch in arches
    ],
)


promotion_ref = ref("./custom/promote")

def promote(ctx):
    print("Promoting")
    return done()

phase(
    name="promote",
    work=[call(
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

def release(ctx):
    version = ctx.inputs.prerelease.split("-")[0]

    # TODO: Implement release logic

    return done(
        outputs={
            "version": version,
        },
        tags=[version],
    )

phase(
    name="release",
    work=[
        call(
            release,
            name="release",
            inputs={
                "prerelease": input(ref="./@/call/increment_version#output/prerelease"),
            },
        ),
    ],
)