const quilt = require('@quilt/quilt');
const infrastructure = require('../../config/infrastructure.js');

/**
 * setHostnames sets the hostnames for `containers` to be a unique hostname
 * prefixed by `hostname`.
 *
 * @param {quilt.Container[]} containers
 * @param {string} hostname
 */
function setHostnames(containers, hostname) {
  containers.forEach((c) => {
    c.setHostname(hostname);
  });
}

const deployment = quilt.createDeployment();
deployment.deploy(infrastructure);

const c = new quilt.Container('alpine', ['tail', '-f', '/dev/null']);

const red = new quilt.Service('red', c.replicate(5));
setHostnames(red.containers, 'red');

const blue = new quilt.Service('blue', c.replicate(5));
setHostnames(blue.containers, 'blue');

const yellow = new quilt.Service('yellow', c.replicate(5));
setHostnames(blue.containers, 'blue');

blue.allowFrom(red, 80);
red.allowFrom(blue, 80);
yellow.allowFrom(red, 80);
yellow.allowFrom(blue, 80);

deployment.deploy(red);
deployment.deploy(blue);
deployment.deploy(yellow);
