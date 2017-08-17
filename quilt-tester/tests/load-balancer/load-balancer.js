const quilt = require('@quilt/quilt');
const infrastructure = require('../../config/infrastructure.js');

const deployment = quilt.createDeployment();
deployment.deploy(infrastructure);

const containers = [];
for (let i = 0; i < 4; i += 1) {
  containers.push(new quilt.Container('nginx:1.10').withFiles({
    '/usr/share/nginx/html/index.html':
        `I am container number ${i.toString()}\n`,
  }));
}

const fetcher = new quilt.Service('fetcher',
  [new quilt.Container('alpine', ['tail', '-f', '/dev/null'])]);
const loadBalanced = new quilt.Service('loadBalanced', containers);
loadBalanced.allowFrom(fetcher, 80);

deployment.deploy([fetcher, loadBalanced]);
