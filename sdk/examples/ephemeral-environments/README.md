# Ephemeral Environments

Ephemeral environments can be created and destroyed on demand. You may want to create one for a pull request to allow testing.

```mermaid
graph LR
    PR(Pull Request 1) --> B(Build)
    B(Build) --> S(Staging PR1)

    M(Merge PR1) --> B2(Build)
    M --> TS((Destroy Staging PR1))
    B2(Build) --> S2(Staging PR1)
    S2 --> P(Production)
```

This is the "happy path" but there will be situations where a pull request is closed or abandoned. So there should probably also be a maximum lifetime for an ephemeral environment.