/* eslint "import/no-extraneous-dependencies":
  ["error", {"devDependencies": true}] */

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
  createDeployment,
  publicInternet,
  resetGlobals,
} = require('./bindings.js');

chai.use(chaiSubset);
const { expect } = chai;

describe('Bindings', () => {
  let deployment;
  beforeEach(() => {
    resetGlobals();
    deployment = createDeployment();
  });

  describe('Machine', () => {
    const checkMachines = function (expected) {
      const { machines } = deployment.toQuiltRepresentation();
      expect(machines).to.have.lengthOf(expected.length)
        .and.containSubset(expected);
    };
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
        id: '0dba3e899652cc7280485c75ad82e7ac7fd6b26e',
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
        id: '0dba3e899652cc7280485c75ad82e7ac7fd6b26e',
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
          id: '4a364f325f7143db589a507cd3defb41a385d1bd',
          role: 'Master',
          provider: 'Amazon',
        },
        {
          id: '6595fa48111eec0c9cef2367f370b20f96d4a38c',
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
          id: '4a364f325f7143db589a507cd3defb41a385d1bd',
          role: 'Master',
          provider: 'Amazon',
          sshKeys: ['key'],
        },
        {
          id: '6595fa48111eec0c9cef2367f370b20f96d4a38c',
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
        id: '632f9edd209f2fcda1e9d407832f169abf66a1a2',
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
        id: '8a0d2198229c09b8b5ec1bdba7105a9e08f8ef0b',
        role: 'Master',
        provider: 'Amazon',
        preemptible: true,
      }]);
    });
  });

  describe('Container', () => {
    const checkContainers = function (expected) {
      const { containers } = deployment.toQuiltRepresentation();
      expect(containers).to.have.lengthOf(expected.length)
        .and.containSubset(expected);
    };
    it('basic', () => {
      deployment.deploy(new Service('foo', [
        new Container('image'),
      ]));
      checkContainers([{
        id: '475c40d6070969839ba0f88f7a9bd0cc7936aa30',
        image: new Image('image'),
        command: [],
        env: {},
        filepathToContent: {},
      }]);
    });
    it('containers are not duplicated', () => {
      const container = new Container('image');
      deployment.deploy(new Service('foo', [container]));
      deployment.deploy(new Service('bar', [container]));
      checkContainers([{
        id: '475c40d6070969839ba0f88f7a9bd0cc7936aa30',
        image: new Image('image'),
        command: [],
        env: {},
        filepathToContent: {},
      }]);
    });
    it('command', () => {
      deployment.deploy(new Service('foo', [
        new Container('image', ['arg1', 'arg2']),
      ]));
      checkContainers([{
        id: '45b5015830c4b8fb738d15c7a2822ee108c20bd8',
        image: new Image('image'),
        command: ['arg1', 'arg2'],
        env: {},
        filepathToContent: {},
      }]);
    });
    it('env', () => {
      const c = new Container('image');
      c.env.foo = 'bar';
      deployment.deploy(new Service('foo', [c]));
      checkContainers([{
        id: 'f54486d0f95b4cc478952a1a775a0699d8f5d959',
        image: new Image('image'),
        command: [],
        env: { foo: 'bar' },
        filepathToContent: {},
      }]);
    });
    it('command, env, and files', () => {
      deployment.deploy(new Service('foo', [
        new Container('image', ['arg1', 'arg2'])
          .withEnv({ foo: 'bar' })
          .withFiles({ qux: 'quuz' }),
      ]));
      checkContainers([{
        id: '3f0028780b8d9e35ae8c02e4e8b87e2ca55305db',
        image: new Image('image'),
        command: ['arg1', 'arg2'],
        env: { foo: 'bar' },
        filepathToContent: { qux: 'quuz' },
      }]);
    });
    it('image dockerfile', () => {
      const c = new Container(new Image('name', 'dockerfile'));
      deployment.deploy(new Service('foo', [c]));
      checkContainers([{
        id: '1dc23744f60b5c2fb7e3eafb3e8a7e3b085b9b9c',
        image: new Image('name', 'dockerfile'),
        command: [],
        env: {},
        filepathToContent: {},
      }]);
    });
    it('replicate', () => {
      deployment.deploy(new Service('foo', new Container('image', ['arg'])
        .replicate(2)));
      checkContainers([
        {
          id: '6563036090dd1a6d4a2fe2f56a31e61c2cdca8e2',
          image: new Image('image'),
          command: ['arg'],
          env: {},
          filepathToContent: {},
        },
        {
          id: 'c6cea5bd411c6e5afac755c37517af89a2d03dbe',
          image: new Image('image'),
          command: ['arg'],
          env: {},
          filepathToContent: {},
        },
      ]);
    });
    it('replicate independent', () => {
      const repl = new Container('image', ['arg']).replicate(2);
      repl[0].env.foo = 'bar';
      repl[0].command.push('changed');
      deployment.deploy(new Service('baz', repl));
      checkContainers([
        {
          id: '6563036090dd1a6d4a2fe2f56a31e61c2cdca8e2',
          image: new Image('image'),
          command: ['arg'],
          env: {},
          filepathToContent: {},
        },
        {
          id: 'c3371b4dec2600f20cd8cc5b59bc116dedcbea92',
          image: new Image('image'),
          command: ['arg', 'changed'],
          env: { foo: 'bar' },
          filepathToContent: {},
        },
      ]);
    });
    it('hostname', () => {
      const c = new Container(new Image('image'));
      c.setHostname('host');
      deployment.deploy(new Service('foo', [c]));
      checkContainers([{
        id: '475c40d6070969839ba0f88f7a9bd0cc7936aa30',
        image: new Image('image'),
        command: [],
        env: {},
        filepathToContent: {},
        hostname: 'host',
      }]);
    });
    it('#getHostname()', () => {
      const c = new Container('image');
      c.setHostname('host');
      expect(c.getHostname()).to.equal('host.q');
    });
    it('duplicate hostname', () => {
      const a = new Container('image');
      a.hostname = 'host';
      const b = new Container('image');
      b.hostname = 'host';
      deployment.deploy(new Service('foo', [a, b]));
      expect(() => deployment.toQuiltRepresentation()).to
        .throw('hostname "host" used for multiple containers');
    });
    it('setHostname generates unique hostnames', () => {
      const a = new Container('image');
      a.setHostname('host');
      const b = new Container('image');
      b.setHostname('host');
      deployment.deploy(new Service('foo', [a, b]));
      checkContainers([{
        id: '475c40d6070969839ba0f88f7a9bd0cc7936aa30',
        image: new Image('image'),
        command: [],
        env: {},
        filepathToContent: {},
        hostname: 'host',
      }, {
        id: '3047630375a1621cb400811b795757a07de8e268',
        image: new Image('image'),
        command: [],
        env: {},
        filepathToContent: {},
        hostname: 'host2',
      }]);
    });
    it('Container.hostname and Service.hostname don\'t conflict', () => {
      const container = new Container('image');
      container.setHostname('foo');
      const serv = new Service('foo', []);
      expect(container.getHostname()).to.equal('foo.q');
      expect(serv.hostname()).to.equal('foo2.q');
    });
  });

  describe('Placement', () => {
    let target;
    const checkPlacements = function (expected) {
      deployment.deploy(target);
      const { placements } = deployment.toQuiltRepresentation();
      expect(placements).to.have.lengthOf(expected.length)
        .and.containSubset(expected);
    };
    beforeEach(() => {
      target = new Service('target', []);
    });
    it('MachineRule size, region, provider', () => {
      target.placeOn({
        size: 'm4.large',
        region: 'us-west-2',
        provider: 'Amazon',
      });
      checkPlacements([{
        targetLabel: 'target',
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
        targetLabel: 'target',
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
        targetLabel: 'target',
        exclusive: false,
        floatingIp: 'xxx.xxx.xxx.xxx',
      }]);
    });
  });
  describe('Label', () => {
    const checkLabels = function (expected) {
      const { labels } = deployment.toQuiltRepresentation();
      expect(labels).to.have.lengthOf(expected.length)
        .and.containSubset(expected);
    };
    it('basic', () => {
      deployment.deploy(
        new Service('web_tier', [new Container('nginx')]));
      checkLabels([{
        name: 'web_tier',
        ids: ['c47b5770b59a4459519ba2b3ae3cd7a1598fbd8d'],
      }]);
    });
    it('multiple containers', () => {
      deployment.deploy(new Service('web_tier', [
        new Container('nginx'),
        new Container('nginx'),
      ]));
      checkLabels([{
        name: 'web_tier',
        ids: [
          'c47b5770b59a4459519ba2b3ae3cd7a1598fbd8d',
          '6044e40ba6e4d97be45ca290b993ef2f368c7bb1',
        ],
      }]);
    });
    it('duplicate services', () => {
      /* Conflicting label names.  We need to generate a couple of dummy
         containers so that the two deployed containers have _refID's
         that are sorted differently lexicographically and numerically. */
      for (let i = 0; i < 2; i += 1) {
        new Container('image');
      }
      deployment.deploy(new Service('foo', [new Container('image')]));
      for (let i = 0; i < 7; i += 1) {
        new Container('image');
      }
      deployment.deploy(new Service('foo', [new Container('image')]));
      checkLabels([
        {
          name: 'foo',
          ids: ['475c40d6070969839ba0f88f7a9bd0cc7936aa30'],
        },
        {
          name: 'foo2',
          ids: ['3047630375a1621cb400811b795757a07de8e268'],
        },
      ]);
    });
    it('get service hostname', () => {
      const foo = new Service('foo', []);
      expect(foo.hostname()).to.equal('foo.q');
    });
    it('get service children', () => {
      const foo = new Service('foo', [
        new Container('bar'),
        new Container('baz'),
      ]);
      expect(foo.children()).to.eql(['1.foo.q', '2.foo.q']);
    });
  });
  describe('AllowFrom', () => {
    let foo;
    let bar;
    beforeEach(() => {
      foo = new Service('foo', []);
      bar = new Service('bar', []);
      deployment.deploy([foo, bar]);
    });
    const checkConnections = function (expected) {
      const { connections } = deployment.toQuiltRepresentation();
      expect(connections).to.have.lengthOf(expected.length)
        .and.containSubset(expected);
    };
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
      expect(() => foo.connect(true, bar)).to
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
    it('allowFrom non-service', () => {
      expect(() => foo.allowFrom(10, 10)).to
        .throw('Services can only connect to other services. ' +
          'Check that you\'re allowing connections from a service, ' +
          'and not from a Container or other object.');
    });
  });
  describe('Connect', () => {
    // This test runs all of the same tests as AllowFrom, but uses the
    // deprecated connect() function rather than the newer allowFrom()
    // function. We can remove this as soon as we remove support for
    // connect().
    let foo;
    let bar;
    beforeEach(() => {
      foo = new Service('foo', []);
      bar = new Service('bar', []);
      deployment.deploy([foo, bar]);
    });
    const checkConnections = function (expected) {
      const { connections } = deployment.toQuiltRepresentation();
      expect(connections).to.have.lengthOf(expected.length)
        .and.containSubset(expected);
    };
    it('port', () => {
      foo.connect(new Port(80), bar);
      checkConnections([{
        from: 'foo',
        to: 'bar',
        minPort: 80,
        maxPort: 80,
      }]);
    });
    it('port range', () => {
      foo.connect(new PortRange(80, 85), bar);
      checkConnections([{
        from: 'foo',
        to: 'bar',
        minPort: 80,
        maxPort: 85,
      }]);
    });
    it('connect to invalid port range', () => {
      expect(() => foo.connect(true, bar)).to
        .throw('Input argument must be a number or a Range');
    });
    it('connect to publicInternet', () => {
      foo.connect(80, publicInternet);
      checkConnections([{
        from: 'foo',
        to: 'public',
        minPort: 80,
        maxPort: 80,
      }]);
    });
    it('connect from publicInternet', () => {
      publicInternet.connect(80, foo);
      checkConnections([{
        from: 'public',
        to: 'foo',
        minPort: 80,
        maxPort: 80,
      }]);
    });
    it('connect to publicInternet port range', () => {
      expect(() => foo.connect(new PortRange(80, 81), publicInternet)).to
        .throw('public internet can only connect to single ports ' +
          'and not to port ranges');
    });
    it('connect from publicInternet port range', () => {
      expect(() => publicInternet.connect(new PortRange(80, 81), foo)).to
        .throw('public internet can only connect to single ports ' +
          'and not to port ranges');
    });
    it('connect to non-service', () => {
      expect(() => foo.connect(10, 10)).to
        .throw('Services can only connect to other services. ' +
          'Check that you\'re connecting to a service, and not ' +
          'to a Container or other object.');
    });
  });
  describe('Vet', () => {
    let foo;
    const deploy = () => deployment.toQuiltRepresentation();
    beforeEach(() => {
      foo = new Service('foo', []);
      deployment.deploy([foo]);
    });
    it('connect to undeployed label', () => {
      foo.allowFrom(new Service('baz', []), 80);
      expect(deploy).to.throw(
        'foo allows connections from an undeployed service: baz');
    });
    it('floating IP and multiple containers', () => {
      foo = new Service('foo', new Container('image').replicate(2));
      foo.placeOn({
        floatingIp: '123',
      });
      foo.connectFromPublic(80);
      deployment.deploy([foo]);
      expect(deploy).to.throw(
        'foo2 has a floating IP and multiple containers. This is ' +
        'not yet supported.');
    });
    it('duplicate image', () => {
      deployment.deploy(new Service('foo',
        [new Container(new Image('img', 'dk'))]));
      deployment.deploy(new Service('foo',
        [new Container(new Image('img', 'dk'))]));
      expect(deploy).to.not.throw();
    });
    it('duplicate image with different Dockerfiles', () => {
      deployment.deploy(new Service('foo',
        [new Container(new Image('img', 'dk'))]));
      deployment.deploy(new Service('foo',
        [new Container(new Image('img', 'dk2'))]));
      expect(deploy).to.throw('img has differing Dockerfiles');
    });
  });
  describe('Custom Deploy', () => {
    it('basic', () => {
      deployment.deploy({
        deploy(dep) {
          dep.deploy([
            new Service('web_tier', [new Container('nginx')]),
            new Service('web_tier2', [new Container('nginx')]),
          ]);
        },
      });
      const { labels } = deployment.toQuiltRepresentation();
      expect(labels).to.have.lengthOf(2)
        .and.containSubset([
          {
            name: 'web_tier',
            ids: ['c47b5770b59a4459519ba2b3ae3cd7a1598fbd8d'],
          },
          {
            name: 'web_tier2',
            ids: ['6044e40ba6e4d97be45ca290b993ef2f368c7bb1'],
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
});
