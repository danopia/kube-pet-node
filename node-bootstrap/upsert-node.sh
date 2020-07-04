#!/bin/bash
set -eux
NodeName="pet-$1"

# get deps
[ -d node_modules ] || npm i

# upsert RBAC
node ./gen-node-identity.js \
  --node-name "$NodeName" \
| kubectl apply -f -

# identify the secret
# TODO: is this racy?
SecretName=$(kubectl get serviceaccount node.${NodeName} \
  --namespace kube-system \
  -o jsonpath='{.secrets[0].name}')

# generate kubeconfig
node ./gen-kubeconfig.js \
  --template <(kubectl config view -o json --raw) \
  --user-name "system:node:${NodeName}" \
  --token <(kubectl get secret ${SecretName} \
    --namespace kube-system \
    -o jsonpath='{.data.token}' \
  | base64 -d) \
> node-kubeconfig.yaml

cat node-kubeconfig.yaml
