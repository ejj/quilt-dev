const quilt = require('@quilt/quilt');
const spark = require('@quilt/spark');
const infrastructure = require('../../config/infrastructure.js');

const deployment = quilt.createDeployment();
deployment.deploy(infrastructure);

const sprk = new spark.Spark(1, 3);

deployment.deploy(sprk);
