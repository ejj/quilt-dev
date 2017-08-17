const quilt = require('@quilt/quilt');
const zookeeper = require('@quilt/zookeeper');
const infrastructure = require('../../config/infrastructure.js');

const deployment = quilt.createDeployment();
deployment.deploy(infrastructure);
deployment.deploy(new zookeeper.Zookeeper(infrastructure.nWorker * 2));
