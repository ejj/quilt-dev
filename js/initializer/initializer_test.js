/* eslint-env mocha */
/* eslint-disable no-unused-expressions, no-underscore-dangle */
const expect = require('chai').expect;
const fs = require('fs');
const fsExtra = require('fs-extra');
const path = require('path');
const os = require('os');
const mock = require('mock-fs');
const rewire = require('rewire');
const sinon = require('sinon');

const consts = require('./constants');

const initializer = rewire('./initializer');
const Provider = rewire('./provider');

describe('Initializer', () => {
  before(() => {
    initializer.__set__('log', sinon.stub());
  });

  describe('writeProviderCreds()', () => {
    let revertConfig;

    beforeEach(() => {
      revertConfig = Provider.__set__('providerConfig', {
        provider: {
          credsTemplate: 'aTemplate',
          credsKeys: {
            key: 'key explanation',
            secret: 'secret explanation',
          },
        },
      });

      mock({
        'js/initializer/templates/aTemplate': '{{key}}-{{secret}}',
        'js/initializer/providers.json': `{
    "provider": {
      "credsLocation": [".some", "file"]
    }
  }`,
      });
    });

    afterEach(() => {
      mock.restore();
      revertConfig();
    });

    const credsSrc = path.join(os.homedir(), '.some', 'src');
    const credsDst = path.join(os.homedir(), '.some', 'file');

    it('creates correct credentials from template', () => {
      const provider = new Provider('provider');

      const credsDest = path.join(os.homedir(), '.some', 'file');
      expect(fs.existsSync(credsDest)).to.be.false;

      fs.mkdir(path.join(os.homedir(), '.some'));
      initializer.writeProviderCreds(
        provider,
        { key: 'myKey', secret: 'mySecret' });
      expect(fs.existsSync(credsDest)).to.be.true;

      expect(fs.readFileSync(credsDest, 'utf8')).to.equal('myKey-mySecret');
    });

    it('creates correct credentials from path', () => {
      const provider = new Provider('provider');

      const copySpy = sinon.spy();
      const fsExtraMock = {
        copy: copySpy,
        mkdirp: fsExtra.mkdirp,
      };
      const revertFs = initializer.__set__('fsExtra', fsExtraMock);

      // The template will be undefined when the provider requires a
      // credentials path instead of template input.
      provider.credsTemplate = undefined;

      initializer.writeProviderCreds(
        provider,
        { [consts.inputCredsPath]: credsSrc });

      expect(copySpy.calledOnce).to.be.true;
      expect(copySpy.getCall(0).args[0]).to.equal(credsSrc);
      expect(copySpy.getCall(0).args[1]).to.equal(credsDst);

      revertFs();
    });

    it('should not create credentials when they are not required', () => {
      const provider = new Provider('provider');

      const requiresCredsStub = sinon.stub(provider, 'requiresCreds');
      requiresCredsStub.returns(false);

      const getCredsPathSpy = sinon.spy(provider, 'getCredsPath');

      initializer.writeProviderCreds(
        provider,
        { [consts.inputCredsPath]: credsSrc });

      expect(getCredsPathSpy.notCalled).to.be.true;

      getCredsPathSpy.restore();
      requiresCredsStub.restore();
    });
  });

  describe('getSshKey()', () => {
    let revertFs;
    let readFileStub;
    let generateSshStub;
    let revertGenerator;

    before(() => {
      readFileStub = sinon.stub();
      readFileStub.returns('');
      const fsMock = {
        readFileSync: readFileStub,
      };
      revertFs = initializer.__set__('fs', fsMock);

      generateSshStub = sinon.stub();
      generateSshStub.returns(true);
      revertGenerator = initializer.__set__(
        'sshGenerateKeyPair', generateSshStub);
    });

    after(() => {
      revertFs();
      revertGenerator();
    });

    afterEach(() => {
      readFileStub.resetHistory();
      generateSshStub.resetHistory();
    });

    it('should not do anything, when the SSH key option is skipped', () => {
      /* eslint-disable no-undef */
      answers = { [consts.sshKeyOption]: consts.skip };
      initializer.getSshKey(answers);
      /* eslint-enable no-undef */
      expect(readFileStub.notCalled).to.be.true;
    });

    it('should generate a new SSH key when relevant', () => {
      initializer.getSshKey({ [consts.sshKeyOption]: consts.sshGenerateKey });
      expect(readFileStub.called).to.be.true;
      expect(generateSshStub.called).to.be.true;
    });

    it('should read an SSH key when given a path', () => {
      const keyPath = 'some/ssh/path';
      initializer.getSshKey({
        [consts.sshKeyOption]: consts.sshUseExistingKey,
        [consts.sshKeyPath]: keyPath,
      });

      expect(generateSshStub.notCalled).to.be.true;
      expect(readFileStub.called).to.be.true;
      expect(readFileStub.getCall(0).args[0]).to.equal(keyPath);
    });

    it('should throw an error when passed an unexpected SSH key option', () => {
      const expectFail = () => {
        initializer.getSshKey({ [consts.sshKeyOption]: 'badOption' });
      };

      expect(expectFail).to.throw();
    });
  });
});
