# Bake Time

This pattern involves leaving a build deployed to a staging environment for a specific amount of time to ensure that it is stable.

## Simple example

```mermaid
graph LR
    C(Commit) --> B(Build)
    B(Build) --> S
    P(Production)

    subgraph Staging
      S(Staging)
      S --> SD{Check passed on this build before?}
      SD -- no --> WS((Wait 1h))
      WS --> SC{Is telemetry good?}
    end
    
    SC -- yes --> P
    SC -- no --> F((Fail build))
    SD -- yes --> P
```

This model implies that the staging deployment will be "in progress" for at least an hour. Builds will continue to stack up while the staging deployment is in progress. Should we even bother to do the builds?

If a later build fails, but there are incomplete builds in between, maybe you could bisect to get some changes out.

## Failing Fast

Avoid congestion when you have a lot of builds stacked up waiting for an hour. Allow fast failures.

```mermaid
graph LR
    C(Commit) --> B(Build)
    B(Build) --> S
    P(Production)

    subgraph Staging
      S(Staging)
      S --> SD{Check passed on this build before?}
      SD -- no --> WS((Wait 5m))
      WS --> SC{Is telemetry good?}
      SC --> SH{An hour of checks?}
      SH -- no --> WS
    end

    SC -- no --> F((Fail build))
    SH -- yes --> P
```

This model will run repeated checks of the telemetry at regular intervals.
This will enable fast failures and unblock replacing with a "good" build.