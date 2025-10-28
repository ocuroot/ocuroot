ocuroot("0.3.0")

initial_prerelease="0.3.11-1"
initial_version="0.3.10"

load("./versions.ocu.star", "next_prerelease_version")

def unit_test():
    print("Running unit tests")
    host.shell("go test ./...")
    return done()

task(unit_test, name="unit_test")

def integration_test():
    print("Running integration tests")
    shell("go test -tags=integration ./...")
    return done()

task(integration_test, name="integration_test")

def endtoend():
    print("Running end-to-end tests")
    host.shell("make e2e")
    return done()

phase(
    name="endtoend",
    work=[call(endtoend, name="endtoend")],
)

def increment_version(ctx):
    prerelease = next_prerelease_version(ctx.inputs.prev_prerelease, ctx.inputs.prev_version)

    create_prerelease(ctx.inputs.prev_version, prerelease)

    return done(
        outputs={
            "prerelease": prerelease,
        },
        tags=[prerelease],
    )

def create_prerelease(previous_version, version):
    # Generate release notes from the git log
    commit_summaries = host.shell("git log v{}..$(git rev-parse HEAD) --pretty='%h %s'".format(previous_version)).stdout
    release_notes = "## Commit summaries\n\n{commit_summaries}".format(
        commit_summaries=commit_summaries,
    )

    # Get the current commit to GH knows what to tag
    target = host.shell("git rev-parse --abbrev-ref HEAD").stdout.strip()

    host.shell("gh release create v{version} --target {target} -p --latest=false --title \"v{version}\" --notes \"$RELEASE_NOTES\"".format(
        version=version,
        target=target,
    ), env={"RELEASE_NOTES": release_notes})

phase(
    name="version",
    work=[call(
        increment_version, 
        name="increment_version", 
        inputs={
            "prev_prerelease": input(ref="./@/task/increment_version#output/prerelease", default=initial_prerelease),
            "prev_version": input(ref="./@/task/release#output/version", default=initial_version),
        },
    )],
)

oses = ["linux", "darwin", "windows"]
arches = ["amd64", "arm64"]

def build(ctx):
    os = ctx.inputs.os
    arch = ctx.inputs.arch
    version = ctx.inputs.version

    # Build the binary for this platform with full version
    output = "./.build/{os}-{arch}/ocuroot".format(os=os, arch=arch)
    host.shell("GOOS={os} GOARCH={arch} go build -o {output} -ldflags=\"-X 'github.com/ocuroot/ocuroot/about.Version={version}'\" ./cmd/ocuroot".format(os=os, arch=arch, output=output, version=version))

    # Build and test packages for Linux platforms
    if os == "linux":
        add_linux_packages_to_release(version, arch)

    add_binary_to_release(version, os, arch)

    return done()

def add_binary_to_release(version, os, arch):
    tar_name = "ocuroot_{os}-{arch}.tar.gz".format(os=os, arch=arch)
    host.shell("tar -czvf .build/{tar_name} -C .build/{os}-{arch} ocuroot".format(os=os, arch=arch, tar_name=tar_name))
    tar_path = "./.build/{tar_name}".format(tar_name=tar_name)
    host.shell("gh release upload v{version} {file}".format(version=version, file=tar_path))

def add_linux_packages_to_release(version, arch):
    # Extract semantic version for package metadata
    semantic_version = version.split("-")[0]
    host.shell("./distribution/build-package.sh {arch} {semantic_version}".format(arch=arch, semantic_version=semantic_version))
    host.shell("./distribution/test/test-package.sh {arch} {semantic_version}".format(arch=arch, semantic_version=semantic_version))
        
    deb_path = "./.build/packages/ocuroot_{semantic_version}_{arch}.deb".format(semantic_version=semantic_version, arch=arch)
    rpm_path = "./.build/packages/ocuroot_{semantic_version}_{arch}.rpm".format(semantic_version=semantic_version, arch=arch)
    host.shell("gh release upload v{version} {deb_path} {rpm_path}".format(
        version=version, 
        deb_path=deb_path, 
        rpm_path=rpm_path
    ))

phase(
    name="build",
    work=[
        call(
            build,
            name="build_{os}_{arch}".format(os=os, arch=arch),
            inputs={
                "os": os,
                "arch": arch,
                "version": input(ref="./task/increment_version#output/prerelease"),
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
            ),

        }    
    )],
)

def release(ctx):
    version = ctx.inputs.prerelease.split("-")[0]
    revision = copy_release(ctx.inputs.prerelease, version)

    return done(
        outputs={
            "version": version,
            "revision": revision,
        },
        tags=[version],
    )

def copy_release(source_tag, target_tag):
    body = shell("gh release view v{source} --json body -q .body".format(source=source_tag)).stdout
    target_hash = shell("git rev-parse HEAD").stdout.strip()   
    shell("gh release create v{target} --target {target_hash} --title \"v{target}\" --notes \"$BODY\"".format(
        target=target_tag,
        target_hash=target_hash,
    ), env={"BODY": body})

    # Download assets from source and upload to target
    shell("rm -rf ./.build/assets/*")
    shell("gh release download v{source} --clobber -p '*.*' -D ./.build/assets/".format(source=source_tag))
    shell("cd ./.build/assets && sha256sum * > checksums.txt")
    shell("gh release upload v{target} ./.build/assets/*.deb ./.build/assets/*.rpm ./.build/assets/*.tar.gz ./.build/assets/checksums.txt".format(target=target_tag))

    return target_hash

def release_inputs():
    inputs = {
        "prerelease": input(ref="./@/task/increment_version#output/prerelease"),
    }
    return inputs

phase(
    name="release",
    work=[
        call(
            release,
            name="release",
            inputs=release_inputs(),
        ),
    ],
)