import {
  KindIdsReq, KindIds, RestClient,
  CoreV1Api, Node, ConfigMap,
  ini, csv,
} from "../deps.ts";

import * as config from "../config.ts";
import { NetworkingConfig, NodeAllocation } from "../types.ts";
import { findNextAllocation } from "../lib/allocations.ts";

function readNetworkingConfig(raw: string) {
  const [topSect, ...sections] = raw.replace(/^\[/gm, '[[').split(/^\[/gm);
  const config = ini.parse(topSect) as NetworkingConfig;
  if (!config.NodeRange) throw new Error(`the networking config is bad`);

  config.Routers = [];
  for (const section of sections) {
    const parsed = ini.parse(section);
    if (parsed.Router) {
      config.Routers.push(parsed.Router);
    }
  }

  return config;
}

// Main loop iteration, given the cluster state
export async function loop(
  restClient: RestClient,
  nodes: (Node & KindIdsReq)[],
  configMaps: (ConfigMap & KindIdsReq)[],
) {
  const coreApi = new CoreV1Api(restClient);

  // Required: core settings ConfigMap
  const sharedCfgMap = configMaps.find(x =>
    x.metadata.name === config.clusterCfgMapName);
  if (!sharedCfgMap) throw new Error(
    `Didn't see a ${config.clusterCfgMapName} ConfigMap, I need it to do anything`);

  const netConf = sharedCfgMap.data?.Networking ? readNetworkingConfig(sharedCfgMap.data.Networking) : null;
  console.log('Found', netConf?.Routers.length, 'routers in cluster config');

  // Optional: dynamic allocations ConfigMap
  let allocationCfgMap: ConfigMap | undefined = configMaps.find(x =>
    x.metadata.name === config.allocationCfgMapName);
  if (!allocationCfgMap && netConf?.WireguardMode === 'SelfProvision') {

    console.log("Bootstrapping empty allocation table...");
    allocationCfgMap = await coreApi.namespace(config.petNamespace).createConfigMap({
      metadata: {
        name: config.allocationCfgMapName,
        labels: config.labelRecord,
        annotations: { [config.purposeAnnotation]: config.cfgPurposeAllocs },
      },
      data: {
        table: `${config.allocFields}\n`,
      }});
  }

  // Gather each node's individual ConfigMap
  const nodeCfgMaps = new Map(configMaps
    .filter(x => (x.metadata.annotations ?? {})[config.purposeAnnotation] === config.cfgPurposeNode)
    .filter(x => x.metadata.ownerReferences?.length === 1 && x.metadata.ownerReferences[0].kind === 'Node')
    .map(x => [x.metadata.ownerReferences![0].name, x]));

  // Work on each node individually.
  for (const node of nodes) {

    const nodeAnnotations = node.metadata.annotations ?? {};
    let nodeCfgMap: ConfigMap | undefined = nodeCfgMaps.get(node.metadata.name);

    // Baby nodes that just showed up
    // ... or known nodes that deleted their own annotations
    if (
      (nodeAnnotations[config.clusterCfgAnnotation] !== resourcePath(sharedCfgMap))
      || (nodeCfgMap && nodeAnnotations[config.nodeCfgAnnotation] !== resourcePath(nodeCfgMap))
    ) {

      // Cluster config tells node how to start configuring networking
      const annotations: Record<string,string> = {
        [config.clusterCfgAnnotation]: resourcePath(sharedCfgMap),
      };
      // If node already exists then let's hand it over in the same go
      if (nodeCfgMap) {
        annotations[config.nodeCfgAnnotation] = resourcePath(nodeCfgMap);
      }

      console.log('Giving node', node.metadata.name, "launch annotations...", Object.keys(annotations));
      await coreApi.patchNode(node.metadata.name, 'strategic-merge', { metadata: { annotations }});

    // Nodes that look ready to have a configmap created
    } else if (!nodeCfgMap && nodeAnnotations[config.wgPubKeyAnnotation] && allocationCfgMap?.data?.table && netConf) {

      const wgPubKey = nodeAnnotations[config.wgPubKeyAnnotation];
      console.log('node', node.metadata.name, 'published pubkey', wgPubKey);

      const allocations = await csv.parse(allocationCfgMap.data.table, { skipFirstRow: true }) as NodeAllocation[];
      let allocation = allocations.find(x => x.NodeKey === wgPubKey);
      if (!allocation) {
        allocation = {
          ...findNextAllocation(netConf, allocations),
          NodeKey: wgPubKey,
          NodeName: node.metadata.name,
        };
        allocations.push(allocation);
        console.log('Storing new allocation:', allocation);

        allocationCfgMap.data.table = await csv
          .stringify(allocations, config.allocFields.split(','))
          .then(x => x.replace(/\r\n/g, `\n`));
        allocationCfgMap = await coreApi
          .namespace(config.petNamespace)
          .replaceConfigMap(allocationCfgMap.metadata!.name!, allocationCfgMap);
      }

      const router = netConf.Routers.find(x => x.PublicKey === allocation?.RouterKey);
      if (!router) throw new Error(`BUG: didn't find router that we just allocated room on`);
      nodeCfgMap = await coreApi
        .namespace(config.petNamespace)
        .createConfigMap({
          metadata: {
            name: 'node-'+node.metadata.name,
            labels: config.labelRecord,
            annotations: {
              [config.purposeAnnotation]: config.cfgPurposeNode,
              [config.wgPubKeyAnnotation]: allocation.NodeKey,
            },
            ownerReferences: [{
              apiVersion: 'v1',
              kind: 'Node',
              name: node.metadata.name,
              uid: node.metadata.uid,
            }],
          },
          data: {
            WireguardConfig:
              `[Interface] # Our own node IP
              Address = ${allocation.NodeIP}/32
              [Peer] # The cluster node/pod IP ranges
              PublicKey = ${allocation.RouterKey}
              AllowedIPs = ${netConf.NodeRange}
              AllowedIPs = ${netConf.PodRange}
              Endpoint = ${router.Endpoint}
              PersistentKeepalive = 25
            `.replace(/^ +/gm, ''),
            IpamJson: JSON.stringify({
              type: "host-local",
              subnet: allocation.PodNet,
              routes: [{ "dst": "0.0.0.0/0" }],
            }),
          }});
      console.log('Wrote new configuration', 'node-'+node.metadata.name);

      // Inform the node of the new configuration available
      const annotations: Record<string,string> = {
        [config.clusterCfgAnnotation]: resourcePath(sharedCfgMap),
        [config.nodeCfgAnnotation]: resourcePath(nodeCfgMap),
      };
      console.log('Giving node', node.metadata.name, "launch annotations...", Object.keys(annotations));
      await coreApi.patchNode(node.metadata.name, 'strategic-merge', { metadata: { annotations }});
    }

  }

}

function resourcePath(res: KindIds) {
  return `${res.metadata?.namespace}/${res.metadata?.name}`;
}
