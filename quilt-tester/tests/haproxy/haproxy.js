const quilt = require('@quilt/quilt');
const hap = require('@quilt/haproxy');
const infrastructure = require('../../config/infrastructure.js');

const indexPath = '/usr/share/nginx/html/index.html';

/**
 * Returns a new Container whose index file contains the given content.
 * @param {string} content - The contents to put in the container's index file.
 * @return {Container} - A container with given content in its index file.
 */
function containerWithContent(content) {
  const files = {};
  files[indexPath] = content;
  return new quilt.Container('nginx').withFiles(files);
}

const serviceA = new quilt.Service('serviceA', [
  containerWithContent('a1'),
  containerWithContent('a2'),
]);

const serviceB = new quilt.Service('serviceB', [
  containerWithContent('b1'),
  containerWithContent('b2'),
  containerWithContent('b3'),
]);

const proxy = hap.withURLrouting(2, {
  'serviceB.com': serviceB,
  'serviceA.com': serviceA,
});

proxy.allowFrom(quilt.publicInternet, 80);

const inf = quilt.createDeployment();

inf.deploy(infrastructure);
inf.deploy(serviceA);
inf.deploy(serviceB);
inf.deploy(proxy);
