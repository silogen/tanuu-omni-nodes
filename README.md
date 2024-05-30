# tanuu-omni-nodes

*Create node groups with Crossplane for Omni*

## Overview

Here we have a go program that is using template yaml files to create actual deployment yaml files which are then used by Crossplane to spin up *worker* clusters.  One part of Crossplane is called "omni" that sets up authentication automagically for the talos virtual machines that consitute the
*worker* kubernets cluster.

Crossplane itself lives in the *ops* cluster that is used to spin up and manage *worker* clusters.

Both *ops* and *worker* clusters live in a tailscale VPN.

TODO: how that operational cluster is created & what are the scripts
to do that & where

## Requirements

Ensure devbox is installed.

Ensure there is an appropriate secret in GCP secrets manager (or depending on your teller config)

Kubeconfig file named `kubeconfig` for a cluster where the crossplane config and claims have been installed. 

## Files

Files at a glance:
```
cmd/                    # the go program
    create/
        templates/      # template yaml file used by the go program
                        # get from silogen platform omni/templates
            kubeconfig.tmpl
                        # kubeconfig for the created cluster
            claim.tmpl
            cluster.tmpl
            # there might be more of these in the future
    menu/
    utils/
logs/                   # devbox task logs in here
kubeconfig              # where to find & how to auth to the ops cluster # TODO: how the credentials are fetched?
.teller.yml             # used by teller to fetch secrets
devbox.json             # tools installed into devbox
main.go                 # go program main entry point
go.mod                  # go module defs
Taskfile.yaml           # used by command `devbox task TASKNAME`
```

Actual deployment files are created from the templates like this:
```
kubeconfig.tmpl --> clustername-kubeconfig          # Where to find & how to auth to the work cluster
claim.tmpl      --> clustername-composition.yaml    # NodeGroupClaim.  Pod types: omni-worker, omni-ctrl.
cluster.tmpl    --> clustername-cluster.yaml        # Cluster, ControlPlane, Workers.  What kind of cluster, controlplane & workers.
```

## Usage

```bash
devbox shell
task omni
```

## Walkthrough

```
"devbox shell"
    "task omni"
        hooks into Taskfile.yaml
            runs create.sh
                runs teller
                    fetches secrets and sets them as env variables
                        OMNI_*
                        TAILSCALE_*
                        GITHUB_TOKEN
                export the file kubeconfig into env variable KUBECONFIG
                runs the go program
                    main.go
```

## Legacy stuff

### Previous version (not recommended)

```bash
devbox shell
task start-test
```
& wait for completion

```bash
kubectl apply --filename instanceclain.yaml
```
