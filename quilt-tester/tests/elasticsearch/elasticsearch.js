const quilt = require('@quilt/quilt');
const Elasticsearch = require('@quilt/elasticsearch').Elasticsearch;
const infrastructure = require('../../config/infrastructure.js');

const deployment = quilt.createDeployment();
deployment.deploy(infrastructure);
deployment.deploy(new Elasticsearch(infrastructure.nWorker).allowFromPublic());
