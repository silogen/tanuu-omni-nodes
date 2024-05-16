# tanuu-omni-nodes
Create node croups with Crossplane for Omni

## Requirements
Ensure devbox is installed!
Ensure there is an appropriate secret in GCP secrets manager (or depending on your teller config)

## Use
Update the cmd/create/templates example files, and rename them to <name>.tmpl
```bash
devbox shell
task omni
```


## Previous version (not recommended)

```bash
devbox shell
task start-test
```

wait for completion

```bash
kubectl apply --filename instanceclain.yaml
```

Enjoy.
