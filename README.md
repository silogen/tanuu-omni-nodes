# tanuu-omni-nodes
Create node croups with Crossplane for Omni

## Requirements
Ensure devbox is installed!
Ensure there is an appropriate secret in GCP secrets manager (or depending on your teller config)

## Install

```bash
devbox shell
task start-test
```

wait for completion

```bash
kubectl apply --filename instanceclain.yaml
```

Enjoy.
