export { autoDetectClient, Reflector } from "https://deno.land/x/kubernetes_client@v0.1.3/mod.ts";
export type { RestClient } from "https://deno.land/x/kubernetes_client@v0.1.3/mod.ts";

export type {
  KindIds,
  KindIdsReq,
} from "https://deno.land/x/kubernetes_client@v0.1.3/reflector.ts";

export {
  CoreV1Api,
} from "https://deno.land/x/kubernetes_apis@v0.2.0/builtin/core@v1/mod.ts";
export type {
  Node,
  ConfigMap,
  ObjectReference,
} from "https://deno.land/x/kubernetes_apis@v0.2.0/builtin/core@v1/mod.ts";

export {
  CertificatesV1beta1Api,
} from "https://deno.land/x/kubernetes_apis@v0.2.0/builtin/certificates.k8s.io@v1beta1/mod.ts";
export type {
  CertificateSigningRequest
} from "https://deno.land/x/kubernetes_apis@v0.2.0/builtin/certificates.k8s.io@v1beta1/mod.ts";

export type {
  Status,
} from "https://deno.land/x/kubernetes_apis@v0.2.0/builtin/meta@v1/structs.ts";


export * as ini from "https://cloudydeno.github.io/ini/ini.ts";

// docs: https://deno.land/std@0.82.0/encoding#csv
export * as csv from "https://deno.land/std@0.82.0/encoding/csv.ts";

export { Curve25519 } from "https://deno.land/x/curve25519@v0.2.0/mod.ts";

export * as Base64 from "https://deno.land/x/base64@v0.2.1/mod.ts";

// docs: https://ip-address.js.org/
export { Address4, Address6 } from "https://cdn.skypack.dev/ip-address@v7.1.0-qifrqZtRtyG5xhdaMNB1?dts";
export { BigInteger } from "https://cdn.skypack.dev/jsbn@v1.1.0-ubfhY6n9xCGJzdkRcjkl?dts";

export * as ows from "./deps-ows.ts";
