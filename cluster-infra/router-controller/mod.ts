import {
  autoDetectClient,
  Reflector,
  CoreV1Api,
  ConfigMap,
  ObjectReference,
  Status,
  KindIdsReq,
  Curve25519,
  Base64,
  ini,
  csv,
  ows,
} from "../deps.ts";

import * as config from "../config.ts";
import { petNamespace, labelSelector } from "../config.ts";
import { NodeAllocation } from "../types.ts";

const ifaceName = 'wg-gke';
const configPath = `/etc/wireguard/${ifaceName}.conf`;
const wgQuickUnit = `wg-quick@${ifaceName}.service`;

interface ConfigSnapshot {
  ConfigMapRef?: ObjectReference,
  PeerBlocks?: string[];
}

function privateToPublic(privKeyStr: string): string {
  const privKey = Base64.decode(privKeyStr);
  if (privKey.length !== 32) throw new Error(`privkey of bad length`);

  const curve = new Curve25519();
  if (curve._9.length !== 32) throw new Error(`basePoint of bad length`);
  curve.selftest();
  curve.selftest();

  const pubKey = curve.scalarMult(privKey, curve._9);
  return Base64.encode(pubKey);
}

const ourPubKey = await Deno
  .readTextFile(configPath)
  .then(x => x.split('[Peer]')[0])
  .then(x => ini.decode(x)['Interface'] as {PrivateKey: string})
  .then(x => privateToPublic(x.PrivateKey))

const restClient = await autoDetectClient();
const coreApi = new CoreV1Api(restClient);

const configStream =
  (Deno.args.includes('--once') ? runOnce() : runForever())
  .pipeThrough(ows.map(buildWgConfig))
  .pipeThrough(ows.distinct((a, b) =>
    JSON.stringify(a) === JSON.stringify(b)))
  .pipeThrough(ows.filter(x =>
    Array.isArray(x.PeerBlocks) ));

// Actually install the latest configuration in a loop
for await (const wgConfig of configStream) {
  const installed = await Deno.readTextFile(configPath);

  const header = installed.split('\n[Peer]')[0];
  const newFile = [ header, ...wgConfig.PeerBlocks! ].join('\n');
  if (installed === newFile) continue;

  await Deno.writeTextFile(configPath, newFile);

  const {code} = await Deno.run({
    cmd: ['systemctl', 'restart', '--', wgQuickUnit],
  }).status();

  // TODO: proper EventBroadcaster analogue, probably.
  // https://github.com/kubernetes/kubernetes/blob/master/staging/src/k8s.io/client-go/tools/events/event_broadcaster.go
  await coreApi.namespace("kube-pets").createEvent({
    metadata: {
      name: 'kube-pet-router-controller.'+Math.random().toString(16).slice(2),
    },
    involvedObject: wgConfig.ConfigMapRef!,
    firstTimestamp: new Date(),
    lastTimestamp: new Date(),
    reason: 'Restarted',
    message: `Reconfigured WireGuard interface `+
      `with ${wgConfig.PeerBlocks?.length} peers. Exit code ${code}`,
    source: {
      component: 'kube-pet-router-controller',
      host: 'pet-omabox',
    },
    count: 1,
    type: code === 0 ? 'Normal' : 'Warning',
  });
}

async function buildWgConfig(items: ConfigMap[]): Promise<ConfigSnapshot> {
  const allocationMap = items.find(x =>
    x.metadata?.annotations![config.purposeAnnotation]
    === config.cfgPurposeAllocs);

  if (typeof allocationMap?.data?.table !== 'string') {
    return { };
  }

  const objRef: ObjectReference = {
    apiVersion: allocationMap.apiVersion,
    kind: allocationMap.kind,
    name: allocationMap.metadata!.name,
    namespace: allocationMap.metadata!.namespace,
    resourceVersion: allocationMap.metadata!.resourceVersion,
    uid: allocationMap.metadata!.uid,
    fieldPath: 'data.table',
  };

  const allocations = await csv
    .parse(allocationMap.data.table, {
      skipFirstRow: true,
    }) as NodeAllocation[];

  // console.log(allocations);
  return {
    ConfigMapRef: objRef,
    PeerBlocks: allocations
      .filter(x => x.RouterKey === ourPubKey)
      .map(x => `
        [Peer] # ${x.NodeName}
        PublicKey = ${x.NodeKey}
        AllowedIPs = ${x.NodeIP}/32 # node
        AllowedIPs = ${x.PodNet} # pods
        `.slice(1).replace(/^ +/gm, '')),
  };
}

// For rapid development, support running without watchers entirely
function runOnce() {
  return ows.just(null)
  .pipeThrough(
    ows.map(() =>
    coreApi
      .namespace(petNamespace)
      .getConfigMapList({ labelSelector })))
  .pipeThrough(ows.map(list => list.items.map(checkKindIds)));
}

// Watch labelled ConfigMaps, only in our namespace
function runForever() {
  const configMapWatcher = new Reflector<ConfigMap,Status>(
    opts => coreApi.namespace(petNamespace)
      .getConfigMapList({ ...opts, labelSelector }),
    opts => coreApi.namespace(petNamespace)
      .watchConfigMapList({ ...opts, labelSelector }));

  configMapWatcher.run();
  console.log('Started Kubernetes Reflector.');

  return ows.fromReflectorCache(configMapWatcher, {
    idleDelayMs: 100,
  });
}


// Helper to make types happy
// Bit of a cop out though...
function checkKindIds<T extends {metadata?: unknown | null}>(input: T): T & KindIdsReq {
  if (!input.metadata) throw new Error(``);
  return input as T & KindIdsReq;
}
