# Approvals

## High Level

Production deploys must wait until the build has been approved.

```mermaid
graph LR
    C(Commit) --> B(Build)
    B(Build) --> S(Staging)
    S --> PD
    subgraph Production
        direction LR
        PD{Was production approved?}
        PD -- no --> P(Production)
        PD -- yes --> W((Wait))
        W --> PD
    end
```

## Multiple approvals

It should be possible to approve multiple environments in bulk, or have specific environments requiring separate approval.

```mermaid
graph LR
    C(Commit) --> B(Build)
    B(Build) --> S(Staging)
    S --> PD3

    S --> A
    A{Was production approved?}
    A -- yes --> P
    A -- yes --> P2
    A -- no --> W((Wait))
    W --> A

    subgraph Production 1
        direction LR
        P(Production)
    end
    
    subgraph Production 2
        direction LR
        P2(Production)
    end

    subgraph Production 3
        direction LR
        PD3{Was production 3 approved?}
        PD3 -- no --> P3(Production)
        PD3 -- yes --> W3((Wait))
        W3 --> PD3
    end
```

## Obtaining approvals

Approvals must be provided by a human. The approval must be tied to a specific build.

It should be possible to control *who* can approve something. This can vary depending on user id, etc.

Approvals should include some kind of signature.

Should approvals be revokable? What happens if an approval is revoked?