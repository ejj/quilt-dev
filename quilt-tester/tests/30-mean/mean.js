const {createDeployment, publicInternet} = require('@quilt/quilt');
let haproxy = require('@quilt/haproxy');
let Mongo = require('@quilt/mongo');
let Node = require('@quilt/nodejs');
let infrastructure = require('../../config/infrastructure.js');

let deployment = createDeployment();
deployment.deploy(infrastructure);

let mongo = new Mongo(3);
let app = new Node({
  nWorker: 3,
  repo: 'https://github.com/tejasmanohar/node-todo.git',
  env: {
    PORT: '80',
    MONGO_URI: mongo.uri('mean-example'),
  },
});

// We should not need to access _app. We will fix this when we decide on a
// general style.
let proxy = haproxy.singleServiceLoadBalancer(3, app._app);

app.connect(mongo.port, mongo);
proxy.allowFrom(publicInternet, haproxy.exposedPort);

deployment.deploy([app, mongo, proxy]);
