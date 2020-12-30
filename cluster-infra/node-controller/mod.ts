import {
  autoDetectClient,
  Reflector,
  CoreV1Api,
  ConfigMap,
  Status,
  KindIdsReq,
  ows,
} from "../deps.ts";

import { labelSelector, petNamespace } from "../config.ts";
import { loop } from "./loop.ts";

const restClient = await autoDetectClient();
const coreApi = new CoreV1Api(restClient);

// For rapid development, support disabling watchers entirely
if (Deno.args.includes('--once')) {
  await runOnce();
} else {
  await runForever();
}

async function runOnce() {
  console.log();

  const [nodeList, configMapList] = await Promise.all([
    coreApi
      .getNodeList({ labelSelector })
      .then(x => x.items.map(checkKindIds)),
    coreApi
      .namespace(petNamespace)
      .getConfigMapList({ labelSelector })
      .then(x => x.items.map(checkKindIds)),
  ]);

  await loop(restClient, nodeList, configMapList);

  Deno.exit(0);
}

async function runForever() {
  // Watch labelled Nodes
  const nodeWatcher = new Reflector(
    opts => coreApi.getNodeList({ ...opts, labelSelector }),
    opts => coreApi.watchNodeList({ ...opts, labelSelector }));

  // Watch labelled ConfigMaps, only in our namespace
  const configMapWatcher = new Reflector<ConfigMap,Status>(
    opts => coreApi.namespace(petNamespace).getConfigMapList({ ...opts, labelSelector }),
    opts => coreApi.namespace(petNamespace).watchConfigMapList({ ...opts, labelSelector }));

  // Actually initiate talking to the cluster
  console.log('Starting reflection sync...');
  nodeWatcher.run();
  configMapWatcher.run();

  // Main loop, given the cluster state
  for await (const [nodes, configMaps] of ows.combineLatest(

    // Nodes, but only rerun on certain changes
    ows.fromReflectorCache(nodeWatcher, {
      idleDelayMs: 250,
      changeFilterKeyFunc: function(node) {
        // What we care about in a node; resists unnecesary main loops
        const annotations = Object.entries(node.metadata?.annotations ?? {})
          .filter(([k]) => !k.startsWith('virtual-kubelet.io/')); // noisey junk
        return [annotations, node.spec];
      },
    }),

    // ConfigMaps
    ows.fromReflectorCache(configMapWatcher, {
      idleDelayMs: 100,
    }),

  ).pipeThrough(ows.debounce(1000))) {
    console.log();
    console.log('---', new Date().toISOString());

    await loop(restClient, nodes, configMaps);

    console.log('---');
    console.log();
  }

  console.log('Completed infinite loop... wait what??');
  Deno.exit(5);
}

// Helper to make types happy
// Bit of a cop out though...
function checkKindIds<T extends {metadata?: unknown | null}>(input: T): T & KindIdsReq {
  if (!input.metadata) throw new Error(``);
  return input as T & KindIdsReq;
}
