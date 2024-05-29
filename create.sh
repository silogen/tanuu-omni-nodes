#!/bin/bash
eval "$(teller sh)"
export KUBECONFIG=kubeconfig
saved_state=$(stty -g)
trap 'stty "$saved_state"' EXIT
# move kubeconfig to file named kubeconfig
go run .
unset KUBECONFIG