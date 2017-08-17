const quilt = require('@quilt/quilt');

const nginx = require('@quilt/nginx');
const infrastructure = require('../../config/infrastructure.js');

const deployment = quilt.createDeployment();
deployment.deploy(infrastructure);

for (let i = 0; i < infrastructure.nWorker; i += 1) {
  deployment.deploy(nginx.createService(80));
  deployment.deploy(nginx.createService(8000));
}
