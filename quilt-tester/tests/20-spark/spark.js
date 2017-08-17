const quilt = require('@quilt/quilt');
const spark = require('@quilt/spark');
const infrastructure = require('../../config/infrastructure.js');

// Application
// sprk.exposeUIToPublic says that the the public internet should be able
// to connect to the Spark web interface. sprk.job causes Spark to run that
// job when it boots.
const sprk = new spark.Spark(1, infrastructure.nWorker - 1)
  .exposeUIToPublic()
  .job('run-example SparkPi');

const deployment = quilt.createDeployment();
deployment.deploy(infrastructure);
deployment.deploy(sprk);
