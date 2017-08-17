/* eslint require-jsdoc: [1] valid-jsdoc: [1] */
const crypto = require('crypto');
const request = require('sync-request');
const stringify = require('json-stable-stringify');
const _ = require('underscore');

const githubCache = {};
/**
  * Return an array of all public SSH keys associated with a GitHub account.
  *
  * @param {string} user A GitHub user name.
  * @return {string[]} An array of public SSH keys.
  */
function githubKeys(user) {
  if (user in githubCache) {
    return githubCache[user];
  }

  const response = request('GET', `https://github.com/${user}.keys`);
  if (response.statusCode >= 300) {
    // Handle any errors.
    throw new Error(
      `HTTP request for ${user}'s github keys failed with error ` +
      `${response.statusCode}`);
  }

  const keys = response.getBody('utf8').trim().split('\n');
  githubCache[user] = keys;

  return keys;
}

// The default deployment object. createDeployment overwrites this.
global._quiltDeployment = new Deployment({});

// The name used to refer to the public internet in the JSON description
// of the network connections (connections to other services are referenced by
// the name of the service, but since the public internet is not a service,
// we need a special label for it).
const publicInternetLabel = 'public';

// Global unique ID counter.
let uniqueIDCounter = 0;

/**
  * Overwrite the deployment object with a new one.
  *
  * @param {Object} deploymentOpts The deployment options.
  * @return {Object} The new deployment object.
  */
function createDeployment(deploymentOpts) {
  global._quiltDeployment = new Deployment(deploymentOpts);
  return global._quiltDeployment;
}

/**
  * Create a deployment object.
  *
  * @param deploymentOpts {Object} The deployment options.
  * @return {Object} The new deployment object.
  */
function Deployment(deploymentOpts) {
  deploymentOpts = deploymentOpts || {};

  this.maxPrice = getNumber('maxPrice', deploymentOpts.maxPrice);
  this.namespace = deploymentOpts.namespace || 'default-namespace';
  this.adminACL = getStringArray('adminACL', deploymentOpts.adminACL);

  this.machines = [];
  this.services = [];
}

/**
  * A replacer function to omit SSH keys.
  *
  * @param {string} key
  * @param {*} value The value corresponding to `key`.
  * @return {*} Returns the given value if `key` is "sshKeys". Else, undefined.
  */
function omitSSHKey(key, value) {
  if (key === 'sshKeys') {
    return undefined;
  }
  return value;
}

/**
  * Return a globally unique integer ID.
  *
  * @return {number} A globally unique integer ID.
  */
function uniqueID() {
  return uniqueIDCounter++;
}

/**
  * Create a key for objects that have a _refID (Containers and Machines).
  *
  * @param {Object} obj An object with a _refID.
  * @return {string} A key.
  */
function key(obj) {
  const keyObj = obj.clone();
  keyObj._refID = '';
  return stringify(keyObj, { replacer: omitSSHKey });
}

/**
  * Deterministically set the id field of objects based on their attributes.
  * The _refID field is required to differentiate between multiple references to
  * the same object, and multiple instantiations with the exact same attributes.
  *
  * @param {Object[]} objs The objects whose ID should be set.
  * @return {void}
  */
function setQuiltIDs(objs) {
  // The refIDs for each identical instance.
  const refIDs = {};
  objs.forEach((obj) => {
    const k = key(obj);
    if (!refIDs[k]) {
      refIDs[k] = [];
    }
    refIDs[k].push(obj._refID);
  });

  // If there are multiple references to the same object, there will be
  // duplicate refIDs.
  Object.keys(refIDs).forEach((k) => {
    refIDs[k] = _.sortBy(_.uniq(refIDs[k]), _.identity);
  });

  objs.forEach((obj) => {
    const k = key(obj);
    obj.id = hash(k + refIDs[k].indexOf(obj._refID));
  });
}

/**
  * Return the SHA1 hash of a string.
  *
  * @param {string} str The string to hash.
  * @return {string} The hex encoded hash.
  */
function hash(str) {
  const shaSum = crypto.createHash('sha1');
  shaSum.update(str);
  return shaSum.digest('hex');
}

/**
  * Convert the deployment to the QRI deployment format.
  *
  * @return {Object} The deployment in the QRI deployment format.
  */
Deployment.prototype.toQuiltRepresentation = function toQuiltRepresentation() {
  this.vet();

  setQuiltIDs(this.machines);

  // List all of the containers in the deployment. This list may contain
  // duplicates; e.g., if the same container is referenced by multiple
  // services.
  const containers = [];
  this.services.forEach((serv) => {
    serv.containers.forEach((c) => {
      containers.push(c);
    });
  });
  setQuiltIDs(containers);

  const services = [];
  let connections = [];
  let placements = [];

  // For each service, convert the associated connections and placement rules.
  // Also, aggregate all containers referenced by services.
  this.services.forEach((service) => {
    connections = connections.concat(service.getQuiltConnections());
    placements = placements.concat(service.placements);

    // Collect the containers IDs, and add them to the container map.
    const ids = [];
    service.containers.forEach((container) => {
      ids.push(container.id);
    });

    services.push({
      name: service.name,
      ids,
    });
  });

  // Create a list of unique containers.
  const addedIds = new Set();
  const containersNoDups = [];
  containers.forEach((container) => {
    if (!addedIds.has(container.id)) {
      addedIds.add(container.id);
      containersNoDups.push(container);
    }
  });

  return {
    machines: this.machines,
    labels: services,
    containers: containersNoDups,
    connections,
    placements,

    namespace: this.namespace,
    adminACL: this.adminACL,
    maxPrice: this.maxPrice,
  };
};

/**
  * Check if all referenced services in connections and placements are
  * really deployed. Throw an error if any check fails.
  *
  * @return {void}
  */
Deployment.prototype.vet = function () {
  const labelMap = {};
  this.services.forEach((service) => {
    labelMap[service.name] = true;
  });

  const dockerfiles = {};
  const hostnames = {};
  this.services.forEach((service) => {
    service.allowedInboundConnections.forEach((conn) => {
      const from = conn.from.name;
      if (!labelMap[from]) {
        throw new Error(`${service.name} allows connections from ` +
          `an undeployed service: ${from}`);
      }
    });

    let hasFloatingIp = false;
    service.placements.forEach((plcm) => {
      if (plcm.floatingIp) {
        hasFloatingIp = true;
      }
    });

    if (hasFloatingIp && service.incomingPublic.length
      && service.containers.length > 1) {
      throw new Error(`${service.name} has a floating IP and ` +
        'multiple containers. This is not yet supported.');
    }

    service.containers.forEach((c) => {
      const name = c.image.name;
      if (dockerfiles[name] != undefined &&
          dockerfiles[name] != c.image.dockerfile) {
        throw new Error(`${name} has differing Dockerfiles`);
      }
      dockerfiles[name] = c.image.dockerfile;

      if (c.hostname !== undefined) {
        if (hostnames[c.hostname]) {
          throw new Error(`hostname "${c.hostname}" used for ` +
            'multiple containers');
        }
        hostnames[c.hostname] = true;
      }
    });
  });
};

/**
  * Add an object, or list of objects, to the deployment.
  * Deployable objects must implement the deploy(deployment) interface.
  *
  * @param {Object|Object[]} toDeployList The objects that should be added to
  *   the deployment.
  * @return {void}
  */
Deployment.prototype.deploy = function deploy(toDeployList) {
  if (toDeployList.constructor !== Array) {
    toDeployList = [toDeployList];
  }

  const that = this;
  toDeployList.forEach((toDeploy) => {
    if (!toDeploy.deploy) {
      throw new Error('only objects that implement ' +
        '"deploy(deployment)" can be deployed');
    }
    toDeploy.deploy(that);
  });
};

function Service(name, containers) {
  if (typeof name !== 'string') {
    throw new Error(`name must be a string; was ${stringify(name)}`);
  }
  if (!Array.isArray(containers)) {
    throw new Error('containers must be an array of Containers (was ' +
      `${stringify(containers)})`);
  }
  containers.forEach((container) => {
    if (!(container instanceof Container)) {
      throw new Error('containers must be an array of Containers; item ' +
        `at index ${i} (${stringify(container)}) is not a ` +
        'Container');
    }
  });
  this.name = uniqueHostname(name);
  this.containers = containers;
  this.placements = [];

  this.allowedInboundConnections = [];
  this.outgoingPublic = [];
  this.incomingPublic = [];
}

/**
  * Return the Quilt hostname that represents the entire service.
  *
  * @return {string} The Quilt hostname for the service.
  */
Service.prototype.hostname = function hostname() {
  return `${this.name}.q`;
};

/**
  * Return a list of Quilt hostnames that address the containers within the
  * service.
  *
  * @return {string[]} The container hostnames.
  */
Service.prototype.children = function () {
  let i;
  const res = [];
  for (i = 1; i < this.containers.length + 1; i += 1) {
    res.push(`${i}.${this.name}.q`);
  }
  return res;
};

/**
  * Deploy this service to the given deployment.
  *
  * @param {Object} deployment The deployment in which to deploy the service.
  * @return {void}
  */
Service.prototype.deploy = function deploy(deployment) {
  deployment.services.push(this);
};

/**
  * Allow this service to initiate traffic to another service on a certain port.
  *
  * @param {Range|number} range The port number or range of port numbers to
  *   allow traffic on.
  * @param {Service} to The receiving service.
  * @return {void}
  */
Service.prototype.connect = function connect(range, to) {
  console.warn('Warning: connect is deprecated; switch to using ' +
    'allowFrom. If you previously used a.connect(5, b), you should ' +
    'now use b.allowFrom(a, 5).');
  if (!(to === publicInternet || to instanceof Service)) {
    throw new Error('Services can only connect to other services. ' +
      'Check that you\'re connecting to a service, and not to a ' +
      'Container or other object.');
  }
  to.allowFrom(this, range);
};

Service.prototype.allowFrom = function (sourceService, portRange) {
  portRange = boxRange(portRange);
  if (sourceService === publicInternet) {
    return this.allowFromPublic(portRange);
  }
  if (!(sourceService instanceof Service)) {
    throw new Error('Services can only connect to other services. ' +
      'Check that you\'re allowing connections from a service, and ' +
      'not from a Container or other object.');
  }
  this.allowedInboundConnections.push(
    new Connection(sourceService, portRange));
};

// publicInternet is an object that looks like another service that can
// allow inbound connections. However, it is actually just syntactic sugar
// to hide the allowOutboundPublic and allowFromPublic functions.
let publicInternet = {
  connect(range, to) {
    console.warn('Warning: connect is deprecated; switch to using ' +
      'allowFrom. Instead of publicInternet.connect(port, service), ' +
      'use service.allowFrom(publicInternet, port).');
    to.allowFromPublic(range);
  },
  allowFrom(sourceService, portRange) {
    sourceService.allowOutboundPublic(portRange);
  },
};

// Allow outbound traffic from the service to public internet.
Service.prototype.connectToPublic = function connectToPublic(range) {
  console.warn('Warning: connectToPublic is deprecated; switch to using ' +
    'allowOutboundPublic.');
  this.allowOutboundPublic(range);
};

Service.prototype.allowOutboundPublic = function allowOutboundPublic(range) {
  range = boxRange(range);
  if (range.min != range.max) {
    throw new Error('public internet can only connect to single ports ' +
      'and not to port ranges');
  }
  this.outgoingPublic.push(range);
};

// Allow inbound traffic from public internet to the service.
Service.prototype.connectFromPublic = function connectFromPublic(range) {
  console.warn('Warning: connectFromPublic is deprecated; switch to ' +
    'allowFromPublic');
  this.allowFromPublic(range);
};

Service.prototype.allowFromPublic = function allowFromPublic(range) {
  range = boxRange(range);
  if (range.min != range.max) {
    throw new Error('public internet can only connect to single ports ' +
      'and not to port ranges');
  }
  this.incomingPublic.push(range);
};

Service.prototype.placeOn = function placeOn(machineAttrs) {
  this.placements.push({
    targetLabel: this.name,
    exclusive: false,
    provider: getString('provider', machineAttrs.provider),
    size: getString('size', machineAttrs.size),
    region: getString('region', machineAttrs.region),
    floatingIp: getString('floatingIp', machineAttrs.floatingIp),
  });
};

Service.prototype.getQuiltConnections = function getQuiltConnections() {
  const connections = [];
  const that = this;

  this.allowedInboundConnections.forEach((conn) => {
    connections.push({
      from: conn.from.name,
      to: that.name,
      minPort: conn.minPort,
      maxPort: conn.maxPort,
    });
  });

  this.outgoingPublic.forEach((rng) => {
    connections.push({
      from: that.name,
      to: publicInternetLabel,
      minPort: rng.min,
      maxPort: rng.max,
    });
  });

  this.incomingPublic.forEach((rng) => {
    connections.push({
      from: publicInternetLabel,
      to: that.name,
      minPort: rng.min,
      maxPort: rng.max,
    });
  });

  return connections;
};

let hostnameCount = {};
function uniqueHostname(name) {
  if (!(name in hostnameCount)) {
    hostnameCount[name] = 1;
    return name;
  }
  hostnameCount[name] += 1;
  return name + hostnameCount[name];
}

// Box raw integers into range.
function boxRange(x) {
  if (x === undefined) {
    return new Range(0, 0);
  }
  if (typeof x === 'number') {
    return new Range(x, x);
  }
  if (!(x instanceof Range)) {
    throw new Error('Input argument must be a number or a Range');
  }
  return x;
}

/**
 * Returns 0 if `arg` is not defined, and otherwise ensures that `arg`
 * is a number and then returns it.
 */
function getNumber(argName, arg) {
  if (arg === undefined) {
    return 0;
  }
  if (typeof arg === 'number') {
    return arg;
  }
  throw new Error(`${argName} must be a number (was: ${stringify(arg)})`);
}

/**
 * Returns an empty string if `arg` is not defined, and otherwise
 * ensures that `arg` is a string and then returns it.
 */
function getString(argName, arg) {
  if (arg === undefined) {
    return '';
  }
  if (typeof arg === 'string') {
    return arg;
  }
  throw new Error(`${argName} must be a string (was: ${stringify(arg)})`);
}

/**
 * Returns an empty array if `arg` is not defined, and otherwise
 * ensures that `arg` is an array of strings and then returns it.
 */
function getStringArray(argName, arg) {
  if (arg === undefined) {
    return [];
  }
  if (!Array.isArray(arg)) {
    throw new Error(`${argName} must be an array of strings ` +
      `(was: ${stringify(arg)})`);
  }
  arg.forEach((elem, i) => {
    if (typeof elem !== 'string') {
      throw new Error(`${argName} must be an array of strings. ` +
        `Item at index ${i} (${stringify(elem)}) is not a ` +
        'string.');
    }
  });
  return arg;
}

/**
 * Returns false if `arg` is not defined, and otherwise ensures
 * that `arg` is a boolean and then returns it.
 */
function getBoolean(argName, arg) {
  if (arg === undefined) {
    return false;
  }
  if (typeof arg === 'boolean') {
    return arg;
  }
  throw new Error(`${argName} must be a boolean (was: ${stringify(arg)})`);
}

function Machine(optionalArgs) {
  this._refID = uniqueID();

  this.provider = getString('provider', optionalArgs.provider);
  this.role = getString('role', optionalArgs.role);
  this.region = getString('region', optionalArgs.region);
  this.size = getString('size', optionalArgs.size);
  this.floatingIp = getString('floatingIp', optionalArgs.floatingIp);
  this.diskSize = getNumber('diskSize', optionalArgs.diskSize);
  this.sshKeys = getStringArray('sshKeys', optionalArgs.sshKeys);
  this.cpu = boxRange(optionalArgs.cpu);
  this.ram = boxRange(optionalArgs.ram);
  this.preemptible = getBoolean('preemptible', optionalArgs.preemptible);
}

Machine.prototype.deploy = function deploy(deployment) {
  deployment.machines.push(this);
};

// Create a new machine with the same attributes.
Machine.prototype.clone = function clone() {
  // _.clone only creates a shallow copy, so we must clone sshKeys ourselves.
  const keyClone = _.clone(this.sshKeys);
  const cloned = _.clone(this);
  cloned.sshKeys = keyClone;
  return new Machine(cloned);
};

Machine.prototype.withRole = function withRole(role) {
  const copy = this.clone();
  copy.role = role;
  return copy;
};

Machine.prototype.asWorker = function asWorker() {
  return this.withRole('Worker');
};

Machine.prototype.asMaster = function asMaster() {
  return this.withRole('Master');
};

// Create n new machines with the same attributes.
Machine.prototype.replicate = function replicate(n) {
  let i;
  const res = [];
  for (i = 0; i < n; i++) {
    res.push(this.clone());
  }
  return res;
};

function Image(name, dockerfile) {
  this.name = name;
  this.dockerfile = dockerfile;
}

Image.prototype.clone = function clone() {
  return new Image(this.name, this.dockerfile);
};

function Container(image, command) {
  // refID is used to distinguish deployments with multiple references to the
  // same container, and deployments with multiple containers with the exact
  // same attributes.
  this._refID = uniqueID();

  this.image = image;
  if (typeof image === 'string') {
    this.image = new Image(image);
  }
  if (!(this.image instanceof Image)) {
    throw new Error('image must be an Image or string (was ' +
      `${stringify(image)})`);
  }

  this.command = getStringArray('command', command);
  this.env = {};
  this.filepathToContent = {};
}

// Create a new Container with the same attributes.
Container.prototype.clone = function clone() {
  const cloned = new Container(this.image.clone(), _.clone(this.command));
  cloned.env = _.clone(this.env);
  cloned.filepathToContent = _.clone(this.filepathToContent);
  return cloned;
};

// Create n new Containers with the same attributes.
Container.prototype.replicate = function replicate(n) {
  let i;
  const res = [];
  for (i = 0; i < n; i += 1) {
    res.push(this.clone());
  }
  return res;
};

Container.prototype.setEnv = function setEnv(key, val) {
  this.env[key] = val;
};

Container.prototype.withEnv = function withEnv(env) {
  const cloned = this.clone();
  cloned.env = env;
  return cloned;
};

Container.prototype.withFiles = function withFiles(fileMap) {
  const cloned = this.clone();
  cloned.filepathToContent = fileMap;
  return cloned;
};

Container.prototype.setHostname = function setHostname(h) {
  this.hostname = uniqueHostname(h);
};

Container.prototype.getHostname = function getHostname() {
  if (this.hostname === undefined) {
    throw new Error('no hostname');
  }
  return `${this.hostname}.q`;
};

function Connection(from, ports) {
  this.minPort = ports.min;
  this.maxPort = ports.max;
  this.from = from;
}

function Range(min, max) {
  this.min = min;
  this.max = max;
}

function Port(p) {
  return new PortRange(p, p);
}

let PortRange = Range;

function getDeployment() {
  return global._quiltDeployment;
}

// Reset global unique counters. Used only for unit testing.
function resetGlobals() {
  uniqueIDCounter = 0;
  hostnameCount = {};
}

module.exports = {
  Container,
  Deployment,
  Image,
  Machine,
  Port,
  PortRange,
  Range,
  Service,
  createDeployment,
  getDeployment,
  githubKeys,
  publicInternet,
  resetGlobals,
};
