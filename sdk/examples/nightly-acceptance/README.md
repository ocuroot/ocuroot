# Nightly Acceptance

Some acceptance runs can take a long time, including performance testing or large numbers of actions.

In these cases, you might want to only run the acceptance test suite once per night.

But you have multiple builds stacked up waiting for the acceptance test suite to run. Which one do you run against and what happens if something goes wrong?

```mermaid
graph LR
    C1(Commit 1) --> B1(Build)
    C2(Commit 2) --> B2(Build)
    C3(Commit 3) --> B3(Build)

    B1 --> S1(Staging)
    B2 --> S2(Staging)
    B3 --> S3(Staging)

    S1 --> ND1{Is midnight and the latest build?}
    S2 --> ND2{Is midnight and the latest build?}
    S3 --> ND3{Is midnight and the latest build?}

    ND3 -- yes --> NB((Nightly Test))

    ND2 -- no --> F2((Stop))
    ND1 -- no --> F1((Stop))

    NB -- pass --> P(Production)
    NB -- fail --> F((Fail build))
    F --> ND2
```

The latest build can be considered to be the most recent successful build for the package.

Failing the build should automatically result in attempting with the next latest build.

The pipelines for Commit 1 and Commit 2 are considered "superceded" unless Commit 3 fails.