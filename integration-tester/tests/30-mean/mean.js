const quilt = require('@quilt/quilt');
const haproxy = require('@quilt/haproxy');
const Mongo = require('@quilt/mongo');
const Node = require('@quilt/nodejs');
const infrastructure = require('../../config/infrastructure.js');

const infra = infrastructure.createTestInfrastructure();

const mongo = new Mongo(3);
const app = new Node({
  nWorker: 3,
  repo: 'https://github.com/quilt/node-todo.git',
  env: {
    PORT: '80',
    MONGO_URI: mongo.uri('mean-example'),
  },
});

const proxy = haproxy.simpleLoadBalancer(app.containers);

mongo.allowFrom(app.containers, mongo.port);
proxy.allowFrom(quilt.publicInternet, haproxy.exposedPort);

app.deploy(infra);
mongo.deploy(infra);
proxy.deploy(infra);
