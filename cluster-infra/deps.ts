export { autoDetectClient, Reflector } from "https://deno.land/x/kubernetes_client@v0.2.4/mod.ts";
export type { RestClient } from "https://deno.land/x/kubernetes_client@v0.2.4/mod.ts";

export type {
  KindIds,
  KindIdsReq,
} from "https://deno.land/x/kubernetes_client@v0.2.4/lib/reflector.ts";

export {
  CoreV1Api,
} from "https://deno.land/x/kubernetes_apis@v0.3.1/builtin/core@v1/mod.ts";
export type {
  Node,
  ConfigMap,
  ObjectReference,
} from "https://deno.land/x/kubernetes_apis@v0.3.1/builtin/core@v1/mod.ts";

export {
  CertificatesV1beta1Api,
} from "https://deno.land/x/kubernetes_apis@v0.3.1/builtin/certificates.k8s.io@v1beta1/mod.ts";
export type {
  CertificateSigningRequest
} from "https://deno.land/x/kubernetes_apis@v0.3.1/builtin/certificates.k8s.io@v1beta1/mod.ts";

export type {
  Status,
} from "https://deno.land/x/kubernetes_apis@v0.3.1/builtin/meta@v1/structs.ts";

// from https://github.com/cloudydeno/deno-bitesized
export * as ini from "https://crux.land/6mMyhY#ini@v1";

export * as Base64 from "https://deno.land/std@0.95.0/encoding/base64.ts";
// docs: https://deno.land/std@0.95.0/encoding#csv
export * as csv from "https://deno.land/std@0.95.0/encoding/csv.ts";

// export { Curve25519 } from "https://deno.land/x/curve25519@v0.2.0/mod.ts";
export { Curve25519 } from "https://crux.land/5K7Q7#curve25519";

// docs: https://ip-address.js.org/
export { Address4, Address6 } from "https://cdn.skypack.dev/ip-address@v7.1.0-qifrqZtRtyG5xhdaMNB1?dts";
export { BigInteger } from "https://cdn.skypack.dev/jsbn@v1.1.0-ubfhY6n9xCGJzdkRcjkl?dts";

export { runMetricsServer } from "https://deno.land/x/observability@v0.1.0/sinks/openmetrics/server.ts";
export { replaceGlobalFetch } from "https://deno.land/x/observability@v0.1.0/sources/fetch.ts";

export * as ows from "./deps-ows.ts";
