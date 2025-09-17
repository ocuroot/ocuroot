# Approvals example

This directory contains example configuration for a release that deploys first to staging, then
requires manual approval before deploying to production.

The entrypoint is `approvals.ocu.star`, which contains the described release workflow. This relies
on `tasks.ocu.star` to define functions to power the release.

Environments are configured in `environments.ocu.star`.

A typical execution would look like:

```bash
# Set up the environments first
$ ocuroot release new environments.ocu.star
approvals:
  ✓ Load config
  environments.ocu.star:
    ✓ Loaded config: No phases
environments:
  +++ production
  +++ production2
  +++ staging

# Kick off the release
$ ocuroot release new approvals.ocu.star
approvals:
  ✓ Load config
  approvals.ocu.star:
    ✓ Loaded config: 2 tasks, deploy to 3 environments
    ✓ Task: build [1] (10.191875ms)
      Outputs
      ├── approvals/-/approvals.ocu.star/@r1/task/build/1#output/output1
      │   └── 5.5
      ├── approvals/-/approvals.ocu.star/@r1/task/build/1#output/output2
      │   └── value2
      ├── approvals/-/approvals.ocu.star/@r1/task/build/1#output/output3
      │   └── true
      └── approvals/-/approvals.ocu.star/@r1/task/build/1#output/output4
          └── 3
    ✓ Deploy to staging [1] (80.064583ms)
      Outputs
      ├── approvals/-/approvals.ocu.star/@r1/deploy/staging/1#output/count
      │   └── 1
      └── approvals/-/approvals.ocu.star/@r1/deploy/staging/1#output/env_name
          └── staging
    › Task: prod_approval [1]
      Pending Inputs
      └── approvals/-/approvals.ocu.star/@r1/custom/approval
    › Deploy to production [1]
    › Deploy to production2 [1]

# Approve promotion to production
$ ocuroot state set approvals/-/approvals.ocu.star/@r1/custom/approval 1
$ ocuroot work any --comprehensive
approvals:
  ✓ Load config
  approvals.ocu.star:
    ✓ Loaded config: 2 tasks, deploy to 3 environments
    ✓ Task: prod_approval [1] (6.488667ms)
    ✓ Deploy to production [1] (75.744083ms)
      Outputs
      ├── approvals/-/approvals.ocu.star/@r1/deploy/production/1#output/count
      │   └── 1
      └── approvals/-/approvals.ocu.star/@r1/deploy/production/1#output/env_name
          └── production
    ✓ Deploy to production2 [1] (83.84675ms)
      Outputs
      ├── approvals/-/approvals.ocu.star/@r1/deploy/production2/1#output/count
      │   └── 1
      └── approvals/-/approvals.ocu.star/@r1/deploy/production2/1#output/env_name
          └── production2
    +++ approvals/-/approvals.ocu.star/@r1/custom/approval
```