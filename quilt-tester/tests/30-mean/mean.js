const {createDeployment} = require('@quilt/quilt');
let HaProxy = require('@quilt/haproxy');
let Mongo = require('@quilt/mongo');
let Node = require('@quilt/nodejs');
let infrastructure = require('../../config/infrastructure.js');

let deployment = createDeployment({});
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
let haproxy = new HaProxy(3, app.services());

mongo.connect(mongo.port, app);
app.connect(mongo.port, mongo);
haproxy.public();

deployment.deploy(app);
deployment.deploy(mongo);
deployment.deploy(haproxy);
