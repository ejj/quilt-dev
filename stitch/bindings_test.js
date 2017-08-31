/* eslint-env mocha */

/* eslint-disable import/no-extraneous-dependencies */
const chai = require('chai');
const chaiSubset = require('chai-subset');
const {
  Container,
  Image,
  Machine,
  Port,
  PortRange,
  Range,
  Service,
  allow,
  createDeployment,
  publicInternet,
  resetGlobals,
  baseInfrastructure,
} = require('./bindings.js');

chai.use(chaiSubset);
const { expect } = chai;

describe('Bindings', () => {
  let deployment;
  beforeEach(() => {
    resetGlobals();
    deployment = createDeployment();
  });

  const checkMachines = function checkMachines(expected) {
    const { machines } = deployment.toQuiltRepresentation();
    expect(machines).to.have.lengthOf(expected.length)
      .and.containSubset(expected);
  };

  const checkContainers = function checkContainers(expected) {
    const { containers } = deployment.toQuiltRepresentation();
    expect(containers).to.have.lengthOf(expected.length)
      .and.containSubset(expected);
  };

  const checkPlacements = function checkPlacements(expected) {
    const { placements } = deployment.toQuiltRepresentation();
    expect(placements).to.have.lengthOf(expected.length)
      .and.containSubset(expected);
  };

  const checkLabels = function checkLabels(expected) {
    const { labels } = deployment.toQuiltRepresentation();
    expect(labels).to.have.lengthOf(expected.length)
      .and.containSubset(expected);
  };

  const checkConnections = function checkConnections(expected) {
    const { connections } = deployment.toQuiltRepresentation();
    expect(connections).to.have.lengthOf(expected.length)
      .and.containSubset(expected);
  };

  describe('Machine', () => {
    it('basic', () => {
      deployment.deploy([new Machine({
        role: 'Worker',
        provider: 'Amazon',
        region: 'us-west-2',
        size: 'm4.large',
        cpu: new Range(2, 4),
        ram: new Range(4, 8),
        diskSize: 32,
        sshKeys: ['key1', 'key2'],
      })]);
      checkMachines([{
        id: 'ae657514e0aa41ed95d9e27c3f3c9b2ff23bd05e',
        role: 'Worker',
        provider: 'Amazon',
        region: 'us-west-2',
        size: 'm4.large',
        cpu: new Range(2, 4),
        ram: new Range(4, 8),
        diskSize: 32,
        sshKeys: ['key1', 'key2'],
      }]);
    });
    it('hash independent of SSH keys', () => {
      deployment.deploy([new Machine({
        role: 'Worker',
        provider: 'Amazon',
        region: 'us-west-2',
        size: 'm4.large',
        cpu: new Range(2, 4),
        ram: new Range(4, 8),
        diskSize: 32,
        sshKeys: ['key3'],
      })]);
      checkMachines([{
        id: 'ae657514e0aa41ed95d9e27c3f3c9b2ff23bd05e',
        role: 'Worker',
        provider: 'Amazon',
        region: 'us-west-2',
        size: 'm4.large',
        cpu: new Range(2, 4),
        ram: new Range(4, 8),
        diskSize: 32,
        sshKeys: ['key3'],
      }]);
    });
    it('replicate', () => {
      const baseMachine = new Machine({ provider: 'Amazon' });
      deployment.deploy(baseMachine.asMaster().replicate(2));
      checkMachines([
        {
          id: '38f289007e41382ce4e2773508609674bac7df52',
          role: 'Master',
          provider: 'Amazon',
        },
        {
          id: 'e23719b2160e4b42c6bbca72567220833fac68da',
          role: 'Master',
          provider: 'Amazon',
        },
      ]);
    });
    it('replicate independent', () => {
      const baseMachine = new Machine({ provider: 'Amazon' });
      const machines = baseMachine.asMaster().replicate(2);
      machines[0].sshKeys.push('key');
      deployment.deploy(machines);
      checkMachines([
        {
          id: '38f289007e41382ce4e2773508609674bac7df52',
          role: 'Master',
          provider: 'Amazon',
          sshKeys: ['key'],
        },
        {
          id: 'e23719b2160e4b42c6bbca72567220833fac68da',
          role: 'Master',
          provider: 'Amazon',
        },
      ]);
    });
    it('set floating IP', () => {
      const baseMachine = new Machine({
        provider: 'Amazon',
        floatingIp: 'xxx.xxx.xxx.xxx',
      });
      deployment.deploy(baseMachine.asMaster());
      checkMachines([{
        id: 'bc2c5392f98b605e90007056e580a42c0c3f960d',
        role: 'Master',
        provider: 'Amazon',
        floatingIp: 'xxx.xxx.xxx.xxx',
        sshKeys: [],
      }]);
    });
    it('preemptible attribute', () => {
      deployment.deploy(new Machine({
        provider: 'Amazon',
        preemptible: true,
      }).asMaster());
      checkMachines([{
        id: '893cfbfaccf6aa6e518f1757dadb07ffb936082f',
        role: 'Master',
        provider: 'Amazon',
        preemptible: true,
      }]);
    });
  });

  describe('Container', () => {
    it('basic', () => {
      deployment.deploy(new Container('host', 'image'));
      checkContainers([{
        id: '293fc7ad8a799d3cf2619a3db7124b0459f395cb',
        image: new Image('image'),
        hostname: 'host',
        command: [],
        env: {},
        filepathToContent: {},
      }]);
    });
    it('containers are not duplicated', () => {
      const container = new Container('host', 'image');
      deployment.deploy([container, container]);
      checkContainers([{
        id: '293fc7ad8a799d3cf2619a3db7124b0459f395cb',
        image: new Image('image'),
        hostname: 'host',
        command: [],
        env: {},
        filepathToContent: {},
      }]);
    });
    it('command', () => {
      deployment.deploy(new Container('host', 'image', {
        command: ['arg1', 'arg2'],
      }));
      checkContainers([{
        id: '9d0d496d49bed06e7c76c2b536d7520ccc1717f2',
        image: new Image('image'),
        command: ['arg1', 'arg2'],
        hostname: 'host',
        env: {},
        filepathToContent: {},
      }]);
    });
    it('env', () => {
      const c = new Container('host', 'image');
      c.env.foo = 'bar';
      deployment.deploy(c);
      checkContainers([{
        id: '299619d3fb4b89fd5cc822983bc3fbcced2f0a98',
        image: new Image('image'),
        command: [],
        env: { foo: 'bar' },
        hostname: 'host',
        filepathToContent: {},
      }]);
    });
    it('hostname', () => {
      const c = new Container('host', new Image('image'));
      deployment.deploy(c);
      expect(c.getHostname()).to.equal('host.q');
      checkContainers([{
        id: '293fc7ad8a799d3cf2619a3db7124b0459f395cb',
        image: new Image('image'),
        command: [],
        env: {},
        filepathToContent: {},
        hostname: 'host',
      }]);
    });
    it('repeated hostnames don\'t conflict', () => {
      const a = new Container('host', 'image');
      const b = new Container('host', 'image');
      deployment.deploy([a, b]);
      checkContainers([{
        id: '293fc7ad8a799d3cf2619a3db7124b0459f395cb',
        image: new Image('image'),
        command: [],
        env: {},
        filepathToContent: {},
        hostname: 'host',
      }, {
        id: '968bcf8c6d235afbc88aec8e1bdddc506714a0b8',
        image: new Image('image'),
        command: [],
        env: {},
        filepathToContent: {},
        hostname: 'host2',
      }]);
    });
    it('Container.hostname and Service.hostname don\'t conflict', () => {
      const container = new Container('foo', 'image');
      const serv = new Service('foo', []);
      expect(container.getHostname()).to.equal('foo.q');
      expect(serv.hostname()).to.equal('foo2.q');
    });
    it('hostnames returned by uniqueHostname cannot be reused', () => {
      const containerA = new Container('host', 'ignoreme');
      const containerB = new Container('host', 'ignoreme');
      const containerC = new Container('host2', 'ignoreme');
      const hostnames = [containerA, containerB, containerC]
        .map(c => c.getHostname());
      const hostnameSet = new Set(hostnames);
      expect(hostnames.length).to.equal(hostnameSet.size);
    });
    it('clone increments existing index if one exists', () => {
      const containerA = new Container('host', 'ignoreme');
      const containerB = containerA.clone();
      const containerC = containerB.clone();
      expect(containerA.getHostname()).to.equal('host.q');
      expect(containerB.getHostname()).to.equal('host2.q');
      expect(containerC.getHostname()).to.equal('host3.q');
    });
    it('duplicate hostname causes error', () => {
      const a = new Container('host', 'image');
      a.hostname = 'host';
      const b = new Container('host', 'image');
      b.hostname = 'host';
      deployment.deploy([a, b]);
      expect(() => deployment.toQuiltRepresentation()).to
        .throw('hostname "host" used multiple times');
    });
    it('image dockerfile', () => {
      const c = new Container('host', new Image('name', 'dockerfile'));
      deployment.deploy(c);
      checkContainers([{
        id: 'fbc9aedb5af0039b8cf09bca2ef5771467b44085',
        image: new Image('name', 'dockerfile'),
        hostname: 'host',
        command: [],
        env: {},
        filepathToContent: {},
      }]);
    });
    it('replicate', () => {
      deployment.deploy(new Container('host', 'image', {
        command: ['arg'],
      }).replicate(2));
      checkContainers([
        {
          id: 'aaf63faa86e552ec4ca75ab66e1b14a5993fa29d',
          image: new Image('image'),
          command: ['arg'],
          hostname: 'host2',
          env: {},
          filepathToContent: {},
        },
        {
          id: '339b2dafcb9fd3c17f01930b5c4782e8d7a9c1b8',
          image: new Image('image'),
          command: ['arg'],
          hostname: 'host3',
          env: {},
          filepathToContent: {},
        },
      ]);
    });
    it('replicate independent', () => {
      const repl = new Container('host', 'image', {
        command: ['arg'],
      }).replicate(2);
      repl[0].env.foo = 'bar';
      repl[0].command.push('changed');
      deployment.deploy(repl);
      checkContainers([
        {
          id: '339b2dafcb9fd3c17f01930b5c4782e8d7a9c1b8',
          image: new Image('image'),
          command: ['arg'],
          hostname: 'host3',
          env: {},
          filepathToContent: {},
        },
        {
          id: 'b318fc1c08ee0a8d74d99f8023112f323268e479',
          image: new Image('image'),
          command: ['arg', 'changed'],
          env: { foo: 'bar' },
          hostname: 'host2',
          filepathToContent: {},
        },
      ]);
    });
  });

  describe('Container attributes', () => {
    const hostname = 'host';
    const image = new Image('image');
    const command = ['arg1', 'arg2'];
    const env = { foo: 'bar' };
    const filepathToContent = { qux: 'quuz' };
    it('with*', () => {
      // The stitch ID is different than the Container created with the
      // constructor because the hostname ID increases with each with*
      // call.
      const id = 'f5c3e0fa3843e6fa149289d476f507831a45654d';
      deployment.deploy(new Container(hostname, image, {
        command,
      }).withEnv(env)
        .withFiles(filepathToContent));
      checkContainers([{
        id,
        image,
        command,
        env,
        filepathToContent,
        hostname: 'host3',
      }]);
    });
    it('constructor', () => {
      const id = '9f9d0c0868163eda5b4ad5861c9558f055508959';
      deployment.deploy(new Container(hostname, image, {
        command, env, filepathToContent,
      }));
      checkContainers([{
        id, hostname, image, command, env, filepathToContent,
      }]);
    });
  });

  describe('Placement', () => {
    let target;
    beforeEach(() => {
      target = new Container('host', 'image');
      deployment.deploy(target);
    });
    it('MachineRule size, region, provider', () => {
      target.placeOn({
        size: 'm4.large',
        region: 'us-west-2',
        provider: 'Amazon',
      });
      checkPlacements([{
        targetContainerID: '293fc7ad8a799d3cf2619a3db7124b0459f395cb',
        exclusive: false,
        region: 'us-west-2',
        provider: 'Amazon',
        size: 'm4.large',
      }]);
    });
    it('MachineRule size, provider', () => {
      target.placeOn({
        size: 'm4.large',
        provider: 'Amazon',
      });
      checkPlacements([{
        targetContainerID: '293fc7ad8a799d3cf2619a3db7124b0459f395cb',
        exclusive: false,
        provider: 'Amazon',
        size: 'm4.large',
      }]);
    });
    it('MachineRule floatingIp', () => {
      target.placeOn({
        floatingIp: 'xxx.xxx.xxx.xxx',
      });
      checkPlacements([{
        targetContainerID: '293fc7ad8a799d3cf2619a3db7124b0459f395cb',
        exclusive: false,
        floatingIp: 'xxx.xxx.xxx.xxx',
      }]);
    });
  });
  describe('Label', () => {
    it('basic', () => {
      deployment.deploy(
        new Service('web_tier', [new Container('host', 'nginx')]));
      checkLabels([{
        name: 'web_tier',
        hostnames: ['host'],
      }]);
    });
    it('multiple containers', () => {
      deployment.deploy(new Service('web_tier', [
        new Container('host', 'nginx'),
        new Container('host', 'nginx'),
      ]));
      checkLabels([{
        name: 'web_tier',
        hostnames: [
          'host',
          'host2',
        ],
      }]);
    });
    it('duplicate services', () => {
      /* Conflicting label names.  We need to generate a couple of dummy
               containers so that the two deployed containers have _refID's
               that are sorted differently lexicographically and numerically. */
      for (let i = 0; i < 2; i += 1) {
        new Container('host', 'image'); // eslint-disable-line no-new
      }
      deployment.deploy(new Service('foo', [
        new Container('host', 'image')]));
      for (let i = 0; i < 7; i += 1) {
        new Container('host', 'image'); // eslint-disable-line no-new
      }
      deployment.deploy(new Service('foo', [
        new Container('host', 'image')]));
      checkLabels([
        {
          name: 'foo',
          hostnames: ['host3'],
        },
        {
          name: 'foo2',
          hostnames: ['host11'],
        },
      ]);
    });
    it('get service hostname', () => {
      const foo = new Service('foo', []);
      expect(foo.hostname()).to.equal('foo.q');
    });
  });
  describe('AllowFrom', () => {
    let foo;
    let bar;
    let fooService;
    beforeEach(() => {
      foo = new Container('foo', 'image');
      fooService = new Service('fooService', [foo]);
      bar = new Container('bar', 'image');
      deployment.deploy([foo, bar, fooService]);
    });
    it('autobox port ranges', () => {
      bar.allowFrom(foo, 80);
      checkConnections([{
        from: 'foo',
        to: 'bar',
        minPort: 80,
        maxPort: 80,
      }]);
    });
    it('port', () => {
      bar.allowFrom(foo, new Port(80));
      checkConnections([{
        from: 'foo',
        to: 'bar',
        minPort: 80,
        maxPort: 80,
      }]);
    });
    it('port range', () => {
      bar.allowFrom(foo, new PortRange(80, 85));
      checkConnections([{
        from: 'foo',
        to: 'bar',
        minPort: 80,
        maxPort: 85,
      }]);
    });
    it('connect to invalid port range', () => {
      expect(() => foo.allowFrom(bar, true)).to
        .throw('Input argument must be a number or a Range');
    });
    it('allow connections to publicInternet', () => {
      publicInternet.allowFrom(foo, 80);
      checkConnections([{
        from: 'foo',
        to: 'public',
        minPort: 80,
        maxPort: 80,
      }]);
    });
    it('allow connections from publicInternet', () => {
      foo.allowFrom(publicInternet, 80);
      checkConnections([{
        from: 'public',
        to: 'foo',
        minPort: 80,
        maxPort: 80,
      }]);
    });
    it('connect to service', () => {
      fooService.allowFrom(bar, 80);
      checkConnections([{
        from: 'bar',
        to: 'fooService',
        minPort: 80,
        maxPort: 80,
      }]);
    });
    it('connect to publicInternet port range', () => {
      expect(() =>
        publicInternet.allowFrom(foo, new PortRange(80, 81))).to
        .throw('public internet can only connect to single ports ' +
                        'and not to port ranges');
    });
    it('connect from publicInternet port range', () => {
      expect(() =>
        foo.allowFrom(publicInternet, new PortRange(80, 81))).to
        .throw('public internet can only connect to single ports ' +
                        'and not to port ranges');
    });
    it('allowFrom non-container', () => {
      expect(() => foo.allowFrom(10, 10)).to
        .throw('Containers can only connect to other containers. ' +
                    'Check that you\'re allowing connections from a ' +
                    'container or list of containers, and not from a Service ' +
                    'or other object.');
    });
  });
  describe('allow', () => {
    let foo;
    let bar;
    let qux;
    let quuz;
    let fooBarGroup;
    let quxQuuzGroup;
    let service;
    beforeEach(() => {
      foo = new Container('foo', 'image');
      bar = new Container('bar', 'image');
      qux = new Container('qux', 'image');
      quuz = new Container('quuz', 'image');

      fooBarGroup = [foo, bar];
      quxQuuzGroup = [qux, quuz];
      service = new Service('serv', [foo, bar, qux, quuz]);

      deployment.deploy(fooBarGroup);
      deployment.deploy(quxQuuzGroup);
      deployment.deploy(service);
    });

    it('both src and dst are lists', () => {
      allow(quxQuuzGroup, fooBarGroup, 80);
      checkConnections([
        { from: 'qux', to: 'foo', minPort: 80, maxPort: 80 },
        { from: 'qux', to: 'bar', minPort: 80, maxPort: 80 },
        { from: 'quuz', to: 'foo', minPort: 80, maxPort: 80 },
        { from: 'quuz', to: 'bar', minPort: 80, maxPort: 80 },
      ]);
    });

    it('dst is a list', () => {
      allow(qux, fooBarGroup, 80);
      checkConnections([
        { from: 'qux', to: 'foo', minPort: 80, maxPort: 80 },
        { from: 'qux', to: 'bar', minPort: 80, maxPort: 80 },
      ]);
    });

    it('src is a list', () => {
      allow(fooBarGroup, qux, 80);
      checkConnections([
        { from: 'foo', to: 'qux', minPort: 80, maxPort: 80 },
        { from: 'bar', to: 'qux', minPort: 80, maxPort: 80 },
      ]);
    });

    it('src is public', () => {
      allow(publicInternet, fooBarGroup, 80);
      checkConnections([
        { from: 'public', to: 'foo', minPort: 80, maxPort: 80 },
        { from: 'public', to: 'bar', minPort: 80, maxPort: 80 },
      ]);
    });

    it('dst is public', () => {
      allow(fooBarGroup, publicInternet, 80);
      checkConnections([
        { from: 'foo', to: 'public', minPort: 80, maxPort: 80 },
        { from: 'bar', to: 'public', minPort: 80, maxPort: 80 },
      ]);
    });

    it('dst is a Service', () => {
      allow(fooBarGroup, service, 80);
      checkConnections([
        { from: 'foo', to: 'serv', minPort: 80, maxPort: 80 },
        { from: 'bar', to: 'serv', minPort: 80, maxPort: 80 },
      ]);
    });
  });
  describe('Vet', () => {
    let foo;
    const deploy = () => deployment.toQuiltRepresentation();
    beforeEach(() => {
      foo = new Service('foo', []);
      deployment.deploy([foo]);
    });
    it('connect from undeployed container', () => {
      foo.allowFrom(new Container('baz', 'image'), 80);
      expect(deploy).to.throw(
        'connection {"from":"baz","maxPort":80,"minPort":80,' +
                '"to":"foo"} references an undefined hostname: baz');
    });
    it('duplicate image', () => {
      deployment.deploy(new Container('host', new Image('img', 'dk')));
      deployment.deploy(new Container('host', new Image('img', 'dk')));
      expect(deploy).to.not.throw();
    });
    it('duplicate image with different Dockerfiles', () => {
      deployment.deploy(new Container('host', new Image('img', 'dk')));
      deployment.deploy(new Container('host', new Image('img', 'dk2')));
      expect(deploy).to.throw('img has differing Dockerfiles');
    });
  });
  describe('Custom Deploy', () => {
    it('basic', () => {
      deployment.deploy({
        deploy(dep) {
          dep.deploy([
            new Service('web_tier', [
              new Container('host', 'nginx')]),
            new Service('web_tier2', [
              new Container('host', 'nginx')]),
          ]);
        },
      });
      const { labels } = deployment.toQuiltRepresentation();
      expect(labels).to.have.lengthOf(2)
        .and.containSubset([
          {
            name: 'web_tier',
            hostnames: ['host'],
          },
          {
            name: 'web_tier2',
            hostnames: ['host2'],
          },
        ]);
    });
    it('missing deploy', () => {
      expect(() => deployment.deploy({})).to.throw(
        'only objects that implement "deploy(deployment)" can be ' +
                'deployed');
    });
  });
  describe('Create Deployment', () => {
    it('no args', () => {
      expect(createDeployment).to.not.throw();
    });
  });
  describe('Query', () => {
    it('namespace', () => {
      deployment = createDeployment({ namespace: 'myNamespace' });
      expect(deployment.toQuiltRepresentation().namespace).to.equal(
        'myNamespace');
    });
    it('default namespace', () => {
      expect(deployment.toQuiltRepresentation().namespace).to.equal(
        'default-namespace');
    });
    it('max price', () => {
      deployment = createDeployment({ maxPrice: 5 });
      expect(deployment.toQuiltRepresentation().maxPrice).to.equal(5);
    });
    it('default max price', () => {
      expect(deployment.toQuiltRepresentation().maxPrice).to.equal(0);
    });
    it('admin ACL', () => {
      deployment = createDeployment({ adminACL: ['local'] });
      expect(deployment.toQuiltRepresentation().adminACL).to.eql(
        ['local']);
    });
    it('default admin ACL', () => {
      expect(deployment.toQuiltRepresentation().adminACL).to.eql([]);
    });
  });
  describe('githubKeys()', () => {});
  describe('baseInfrastructure', () => {
    it('should error if name is not a string', () => {
      const expectedFail = () => {
        baseInfrastructure(1);
      };
      expect(expectedFail).to.throw('name must be a string');
    });
  });
});
