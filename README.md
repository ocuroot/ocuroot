# ocuroot

The Ocuroot client. The client is a command line tool that manages state for releases and environments of your
applications and cloud resources. It is provided as a standalone tool that runs on top of existing CI services.

## About Ocuroot

Ocuroot is a CI/CD orchestration tool that gives you more control over complex release pipelines. Rather than
designing DAGs, Ocuroot starts from higher level concepts like deployments and environments. You can also configure
your release process using imperative code for a high degree of flexibility.

Core to Ocuroot are the concepts of *state* and *intent*. State represents the known, deployed state of your
applications and resources. Intent represents the desired state that you want Ocuroot to effect, allowing
GitOps-like workflows.

There are 4 key elements in state:
* *Releases* define a process to build and deploy your applications and resources
* *Environments* define target locations where your applications and resources are deployed
* *Deployments* represent a specific release being deployed to an environment
* *Custom state* allows you to pass data into releases and deployments without having to modify code

## Installation

### From Source

```bash
go install github.com/ocuroot/ocuroot@latest
```

## Configuration

All configuration written in imperative [Starlark](https://github.com/bazelbuild/starlark),
a dialect of Python. Ocuroot uses the `.ocu.star` suffix to distinguish its configuration files from other source.

### The SDK

The SDK provides functions and structs to interact with Ocuroot, as well as the host system and network.

At the top of each `.ocu.star` file, you specify the version of the SDK you want to use:

```python
ocuroot("0.3.0")
```

A full set of stubs for the 0.3.0 SDK can be found at [sdk/sdk/0.3.0](sdk/sdk/0.3.0).   

### repo.ocu.star

The `repo.ocu.star` file must be placed in the root of your repo. It defines common configuration used by all
other config files.

A *repo alias* may be defined to override the default repo name. This name would be used in all state references
to content within this repo.

```python
repo_alias("my_repo")
```

A location for the *state store* should be set so Ocuroot knows where to read and write state and intent. Most
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

Releases describe a deployment process end-to-end, they can be included in any `*.ocu.star` file.

Releases are divided into Phases, which are executed in-order. Within each Phase are a set of Work items
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

This will start a new Release with the current state of your Git repo for the Release process defined in `my_release.ocu.star`. Once a Release has been started, Ocuroot will execute its Phases in-order as long as it is able to do so.

### Managing work

Sometimes, a dependency will not allow a Release to continue, and you need to come back later. You can pick up any
outstanding work on a given commit in your Git repo by running:

```bash
ocuroot work continue
```

If you want to kick of outstanding work in your CI platform, you can use the trigger command:

```bash
ocuroot work trigger
```

## Examples

You can find examples of Ocuroot configuration in the [examples](examples) directory.

There are also complete example repos that can be cloned and experimented with:

* [k8s-demo](https://github.com/ocuroot/k8s-demo)