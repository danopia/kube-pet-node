#FROM hayd/alpine-deno:1.6.1
FROM danopia/deno-experiments:per_op_metrics
WORKDIR /src/kube-pet-node/cluster-info

ADD deps.ts deps-ows.ts ./
RUN ["deno", "cache", "deps.ts"]

ADD . ./
RUN ["deno", "cache", "node-controller/mod.ts"]

ENTRYPOINT ["deno", "run", "--unstable", "--allow-net", "--allow-hrtime", "--allow-read=/var/run/secrets/kubernetes.io", "--allow-env", "--cached-only", "--no-check"]
