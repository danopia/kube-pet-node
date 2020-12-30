export const petNamespace = "kube-pets";

export const labelRecord: Record<string,string> = {"kubernetes.io/role": "pet"};
export const labelSelector = "kubernetes.io/role=pet";

export const clusterCfgMapName = "kube-pet-config";
export const allocationCfgMapName = "kube-pet-allocations";

export const clusterCfgAnnotation = "pet.wg69.net/cluster-configmap";
export const nodeCfgAnnotation = "pet.wg69.net/node-configmap";

export const wgPubKeyAnnotation = "pet.wg69.net/wg-pubkey";

export const purposeAnnotation = "pet.wg69.net/purpose";
export const cfgPurposeCluster = "cluster configuration";
export const cfgPurposeNode = "node configuration";
export const cfgPurposeAllocs = "address allocation table";

export const allocFields = "NodeKey,RouterKey,NodeName,NodeIP,PodNet";
