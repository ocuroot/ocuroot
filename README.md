# ocuroot

The Ocuroot client. The client is a command line tool that manages state for
releases and environments of your applications and cloud resources. It's provided
as a standalone tool that runs on top of existing CI services.

## About Ocuroot

Ocuroot is a CI/CD orchestration tool that gives you more control over complex release pipelines. Rather than
designing DAGs, Ocuroot starts from higher level concepts like deployments and environments. You can also configure
your release process using imperative code for a high degree of flexibility.

Core to Ocuroot are the concepts of *state* and *intent*. State represents the known, deployed state of your
applications and resources. Intent represents the desired state that you want Ocuroot to effect, allowing
GitOps-like workflows.

There are 4 key elements in state:

* *Releases* define a process to build and deploy your applications and resources
* *Environments* define locations where releases are deployed
* *Deployments* represent a specific release being deployed to an environment
* *Custom State* allows you to pass data into releases and deployments without having to modify code

Of these three, Environments, Deployments and Custom State have intent equivalents so they can be manually modified.
Releases are entirely managed by Ocuroot based on the contents of your source repo.

## Installation

### From Source

```bash
go install github.com/ocuroot/ocuroot@latest
```

## Configuration

All configuration is written in imperative [Starlark](https://github.com/bazelbuild/starlark),
a dialect of Python. Ocuroot uses the `.ocu.star` suffix to distinguish its
configuration files.

### State

All state managed by Ocuroot is stored as JSON documents and organized by Reference, a URI-compatible string
that describes config files within source repos, Releases, Deployments, Environments and Custom State.

State can be queried and manipulated using the `ocuroot state` commands. Run `ocuroot state --help` for more
information

References are of the form:

```
[repo]/-/[path]/@[release]/[subpath]#[fragment]
```

* [repo]: Is the URL or alias of a Git repo.
* [path]: Is the path to a file within the repo, usually a *.ocu.star file.
* [release]: Is a release identifier. If blank, the most recent release is implied.
* [subpath]: A path to a document within the release, such as a deployment to a specific environment.
* [fragment]: An optional path to a field within the document.

For example, `github.com/ocuroot/example/-/frontend/release.ocu.star/@1.0.0/call/build#output/image` would
refer to the container image for the 1.0.0 release of the frontend in an example repo.

Intent References are denpted by the use of `+` instead of `@` for the release. So
`github.com/ocuroot/example/-/frontend/release.ocu.star/+/deploy/production` would
refer to the desired state for deploying the frontend to the production environment.

### The SDK

The SDK provides functions and structs to interact with Ocuroot, as well as the host system and network.

At the top of each `.ocu.star` file, you specify the version of the SDK you want to use:

```python
ocuroot("0.3.0")
```

A full set of stubs for the 0.3.0 SDK can be found at [sdk/sdk/0.3.0](sdk/sdk/0.3.0).

### repo.ocu.star

The `repo.ocu.star` file defines common configuration used by all other config files.
It must be placed in the root of your repo.

A *repo alias* may be defined to override the default repo name. This name would be used in all state references
to content within this repo.

```python
repo_alias("my_repo")
```

A location for your *state store* must be set so Ocuroot knows where to read and write state and intent. Most
commonly, this will be one or two git repos:

```python
store.set(
    store.git("ssh://git@github.com/ocuroot/ocuroot-state.git"),
    store.git("ssh://git@github.com/ocuroot/ocuroot-intent.git"),
)
```

Finally, you can define a *trigger function* that can be called to schedule work on your CI platform.

```python
def _trigger(commit):
    # Trigger code goes here
    # ...
    pass

trigger(_trigger)
```

### Defining releases

Releases describe deployment processes end-to-end. They can be included in any *.ocu.star file.

Releases are divided into Phases, which are executed in order. Within each Phase are a set of Work items,
which may be calls to functions or deployments. These items may be executed concurrently within each Phase.

```python
def build(ctx):
    # ... build code goes here ...

    # All functions for call or deploy must return done when complete.
    return done()

phase(
    name="build",
    work=[call(build, name="build")],
)

envs = environments()

def up(ctx):
    print("Deploying to " + ctx.inputs.environment["name"])
    # ... deploy code goes here ...

    return done()

def down(ctx):
    print("Tearing down " + ctx.inputs.environment["name"])
    # ... teardown code goes here ...

    return done()

phase(
    name="deploy",
    work=[
        deploy(
            up=up, # Executed when deploying a release to this environment
            down=down, # Executed when tearing down a release
            environment=environment,
        ) for environment in envs # All environments may be deployed concurrently
    ],
)
```

### Environments

You may have noticed the call to `environments()` in the above example. This function returns a list of
all Environments that have been registered in Ocuroot state. You can define Environments in any `*.ocu.star`
file as with a Release.

```python
register_environment(environment("staging", {"type": "staging"}))
register_environment(environment("production", {"type": "prod"}))
register_environment(environment("production2", {"type": "prod"}))
```

Calling `register_environment` sets up a Release to generate these environments.

## Usage

### Creating a Release

You can start a new Release by running:

```bash
ocuroot release new my_release.ocu.star
```

This will start a new Release using the process defined in `my_release.ocu.star` at the current commit in your source repo. 
Once a Release has been started, Ocuroot will execute its Phases in order as long as it's able to do so.

### Managing work

Sometimes, a dependency won't allow a Release to continue. For example, a frontend service may have a dependency on a backend
service that has not yet been deployed.

Once your dependencies are satisfied, you can pick up any outstanding work against your currently checked out commit by running:

```bash
ocuroot work continue
```

The above will continue work on your local machine. If you want to schedule any outstanding work onto your CI platform, 
you can use the trigger command:

```bash
ocuroot work trigger
```

## Examples

You can find examples of Ocuroot configuration in the [examples](examples) directory.

There are also complete example repos that can be cloned and experimented with:

* [k8s-demo](https://github.com/ocuroot/k8s-demo)

