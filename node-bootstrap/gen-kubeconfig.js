import fs from 'fs'
import YAML from 'yaml'

const argTemplName = '--template';
const argTemplIdx = process.argv.indexOf(argTemplName) + 1;
const argUserName = '--user-name';
const argUserIdx = process.argv.indexOf(argUserName) + 1;
const argTokenName = '--token';
const argTokenIdx = process.argv.indexOf(argTokenName) + 1;
if (Math.min(argTemplIdx, argUserIdx, argTokenIdx) < 1 ||
    Math.max(argTemplIdx, argUserIdx, argTokenIdx) >= process.argv.length) {
  console.error(`Usage:`, 'gen-kubeconfig.js',
    `--template <template-json-path>`,
    `--user-name <username-for-kubeconfig>`,
    `--token <raw-token-path>`);
  console.error();
  process.exit(12);
}
const templatePath = process.argv[argTemplIdx];
const userName = process.argv[argUserIdx];
const tokenPath = process.argv[argTokenIdx];

const template = JSON.parse(fs.readFileSync(templatePath, 'utf-8'));
const currentCtx = template.contexts.find(x => x.name === template['current-context']).context;
const clusterCfg = template.clusters.find(x => x.name === currentCtx.cluster);

const token = fs.readFileSync(tokenPath, 'utf-8');

console.log(YAML.stringify({
  apiVersion: 'v1',
  kind: 'Config',

  'current-context': 'pet-node-sa',
  contexts: [{
    context: {
      cluster: clusterCfg.name,
      user: userName,
    },
    name: 'pet-node-sa',
  }],

  clusters: [clusterCfg],
  users: [{
    name: userName,
    user: { token },
  }],

  preferences: {},
}));
