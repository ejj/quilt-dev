const { createDeployment } = require('@quilt/quilt');
const infrastructure = require('./infrastructure.js');

createDeployment({}).deploy(infrastructure);
