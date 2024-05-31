# tanuu-omni-nodes

*Create custom Kubernetes clusters with Crossplane and Omni*

## Overview

Custom Kubernetes clusters, based on talos linux images, are created with Crossplane and Omni into a tailscale VPN.

Crossplane and Omni live in the *ops* cluster and they are used to spin up and manage *worker* clusters.

A small go program is used to automatize all steps.

## Requirements

Ensure devbox is installed.

Ensure there is an appropriate secret in GCP secrets manager (or depending on your teller config)

Kubeconfig file named `kubeconfig` for the *ops* cluster where the crossplane config and claims have been installed. 

## Files

Files at a glance:
```
cmd/                    # go programs subroutines
    create/
        create.go       # creates deployment files from templates and applies them (**)
        templates/      # template yaml file used by the go program
                        # get from silogen platform omni/templates
            kubeconfig.tmpl
                        # kubeconfig for the created cluster
            claim-base.tmpl
            claim-gpu.tml
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

Actual deployment files are created from the templates (**) like this:
```
claim.tmpl      --> TAG-composition.yaml            # NodeGroupClaim.  Image types: omni-worker, omni-ctrl.
                                                    # uses kubectl apply (**) to activate Crossplane
                                                    # Images based on talos linux

cluster.tmpl    --> TAG-cluster.yaml                # Cluster, ControlPlane, Workers.  What kind of cluster, controlplane & workers.
                                                    # Used with `omnictl` (**) to tell omni what kind of kubernets cluster we'll have.

kubeconfig.tmpl --> TAG-kubeconfig                  # API endpoint to the work cluster
```
For more details, see below "Walkthroughs".

## Usage

Run
```bash
gcloud auth application-default login
devbox shell
task omni
```

Cleanup
```
rm *-cluster.yaml *-composition.yaml *.kubeconfig
```

## Walkthroughs

Running `task omni`
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

The go program
```
Apply TAG-composition.yaml (see above) to Crossplane (running in the ops cluster)
    Creates VMs in the cloud provider with talos linux images that have OMNI URL 
    and certificates baked in
    --> VMs (with name TAG) spin up & connect to OMNI URL let's call them OMNI VMs

Check with omni if all OMNI VMs with name TAG are available

Create TAG-cluster.yaml (see above) & apply it with command `omnictl`
    Omni creates the kubernetes cluster and tells the VMs to config
    themselves as part of the cluster with a specific config as described
    by TAG-cluster.yaml
        Each omni cluster applies also by itself TAG-cluster.yaml
        in `extraManifests` for example the tailscale connection is described
        Each omni cluster boots according to those extra configs

Now there is a kubernets cluster within the tailscale network!

Create TAG.kubeconfig with the correct address of the kubernetes cluster within 
tailscale network
```
