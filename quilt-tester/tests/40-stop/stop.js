const quilt = require('@quilt/quilt');
const infrastructure = require('../../config/infrastructure.js');

const deployment = quilt.createDeployment();
deployment.deploy(infrastructure);

const containers = new quilt.Service('containers',
  new quilt.Container('google/pause').replicate(infrastructure.nWorker));
deployment.deploy(containers);
