import YAML from 'yaml'

const argName = '--node-name';
const argIdx = process.argv.indexOf(argName) + 1;
if (argIdx < 1 || argIdx >= process.argv.length) {
  console.error(`Usage:`, 'gen-node-identity.js', `--node-name <node-name>`);
  console.error();
  process.exit(12);
}
const nodeName = process.argv[argIdx];
const labels = {
  'cluster-node': nodeName,
};

console.log('---')
console.log(YAML.stringify({
  apiVersion: 'v1',
  kind: 'ServiceAccount',
  metadata: {
    name: `node.${nodeName}`,
    namespace: 'kube-system',
    labels,
  },
}));

console.log('---')
console.log(YAML.stringify({
  apiVersion: 'rbac.authorization.k8s.io/v1beta1',
  kind: 'ClusterRoleBinding',
  metadata: {
    name: `system:node:${nodeName}`,
    labels,
  },
  roleRef: {
    apiGroup: `rbac.authorization.k8s.io`,
    kind: `ClusterRole`,
    name: `system:node`,
  },
  subjects: [{
    kind: `ServiceAccount`,
    name: `node.${nodeName}`,
    namespace: `kube-system`,
  }],
}));
