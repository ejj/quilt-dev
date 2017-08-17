const quilt = require('@quilt/quilt');
const infrastructure = require('../../config/infrastructure.js');

const deployment = quilt.createDeployment();
deployment.deploy(infrastructure);

const red = new quilt.Service('red', [new quilt.Container('google/pause')]);
deployment.deploy(red);
