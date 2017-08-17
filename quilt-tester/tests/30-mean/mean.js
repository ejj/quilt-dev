const quilt = require('@quilt/quilt');
const haproxy = require('@quilt/haproxy');
const Mongo = require('@quilt/mongo');
const Node = require('@quilt/nodejs');
const infrastructure = require('../../config/infrastructure.js');

const deployment = quilt.createDeployment();
deployment.deploy(infrastructure);

const mongo = new Mongo(3);
const app = new Node({
  nWorker: 3,
  repo: 'https://github.com/tejasmanohar/node-todo.git',
  env: {
    PORT: '80',
    MONGO_URI: mongo.uri('mean-example'),
  },
});

// We should not need to access _app. We will fix this when we decide on a
// general style.
const proxy = haproxy.singleServiceLoadBalancer(3, app._app);

mongo.allowFrom(app, mongo.port);
proxy.allowFrom(quilt.publicInternet, haproxy.exposedPort);

deployment.deploy([app, mongo, proxy]);
