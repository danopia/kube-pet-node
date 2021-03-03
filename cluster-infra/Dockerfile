# FROM hayd/alpine-deno:1.8.0
FROM danopia/deno-experiments:1.8.0-heapmetrics
WORKDIR /src/kube-pet-node/cluster-info

ADD deps.ts deps-ows.ts ./
RUN ["deno", "cache", "deps.ts"]

ADD . ./
RUN ["deno", "cache", "node-controller/mod.ts"]

ENTRYPOINT ["deno", "run", "--unstable", "--allow-net", "--allow-hrtime", "--allow-read=/var/run/secrets/kubernetes.io", "--allow-env", "--cached-only", "--no-check"]
