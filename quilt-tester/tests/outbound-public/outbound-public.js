const quilt = require('@quilt/quilt');
const infrastructure = require('../../config/infrastructure.js');

const deployment = quilt.createDeployment();
deployment.deploy(infrastructure);

const connected = new quilt.Service('connected',
  new quilt.Container('alpine', ['tail', '-f', '/dev/null'])
    .replicate(infrastructure.nWorker * 2));
quilt.publicInternet.allowFrom(connected, 80);

const notConnected = new quilt.Service('not-connected',
  new quilt.Container('alpine', ['tail', '-f', '/dev/null'])
    .replicate(infrastructure.nWorker * 2));

deployment.deploy([connected, notConnected]);
