FROM hayd/alpine-deno:1.6.1

WORKDIR /src/kube-pet-node/cluster-info
ADD . ./
RUN deno cache node-controller/mod.ts

ENTRYPOINT ["deno", "run", "--allow-net", "--allow-read", "--allow-env", "--cert=/var/run/secrets/kubernetes.io/serviceaccount/ca.crt", "--cached-only"]
