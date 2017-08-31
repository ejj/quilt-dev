/* eslint-env mocha */
/* eslint-disable no-unused-expressions, no-underscore-dangle */
const expect = require('chai').expect;
const mock = require('mock-fs');
const os = require('os');
const path = require('path');
const fs = require('fs');
const rewire = require('rewire');

const Provider = rewire('./provider');

describe('Provider', () => {
  let revertConfig;

  before(() => {
    revertConfig = Provider.__set__('providerConfig', {
      providerA: {
        credsTemplate: 'aTemplate',
        credsKeys: {
          key: 'aKey',
          secret: 'aSecret',
        },
        requiresSsh: true,
      },
      providerB: {
        requiresSsh: false,
      },
      providerC: {},
    });

    mock({
      'quiltctl/command/init/providers.json': `{
  "providerA": {
    "sizes": {
      "small": "size1",
      "medium": "size2",
      "large": "size3"
    },
    "regions": [
      "reg1",
      "reg2"
    ],
    "hasPreemptible": true,
    "credsLocation": [".some", "file"]
  },
  "providerB": {
    "hasPreemptible": false
  },
  "providerC": {
    "credsLocation": [".another", "file"]
  }
}`,
    });
  });

  after(() => {
    mock.restore();
    revertConfig();
  });

  let providerA;
  let providerB;
  let providerC;

  describe('getName()', () => {
    it('should get name for provider A', () => {
      providerA = new Provider('providerA');
      expect(providerA.getName()).to.equal('providerA');
    });

    it('should get name for provider B', () => {
      providerB = new Provider('providerB');
      expect(providerB.getName()).to.equal('providerB');
    });
  });

  describe('getCredsKeys()', () => {
    it('should get the keys', () => {
      expect(providerA.getCredsKeys()).to.deep.equal({
        key: 'aKey',
        secret: 'aSecret',
      });
    });

    it('should not break when there are no keys', () => {
      expect(providerB.getCredsKeys()).to.deep.equal({});
    });
  });

  describe('getRegions()', () => {
    it('should return correct regions', () => {
      expect(providerA.getRegions()).to.deep.equal(['reg1', 'reg2']);
    });

    it('should not break when there are no regions', () => {
      expect(providerB.getRegions()).to.deep.equal([]);
    });
  });

  describe('getCredsTemplate()', () => {
    it('should return the name of the template file', () => {
      expect(providerA.getCredsTemplate()).to.equal('aTemplate');
    });

    it('should not break when there is no template', () => {
      expect(providerB.getCredsTemplate()).to.equal('');
    });
  });

  describe('requiresCreds()', () => {
    it('should return true if the provider requires credentials', () => {
      expect(providerA.requiresCreds()).to.be.true;
    });

    it('should return false if the provider does not require credentials',
      () => {
        expect(providerB.requiresCreds()).to.be.false;
      });
  });

  const credentialsPathA = path.join(os.homedir(), '.some/file');
  const credentialsPathC = path.join(os.homedir(), '.another/file');
  describe('credsExist()', () => {
    it('should return false if there are no existing credentials', () => {
      expect(providerA.credsExist()).to.be.false; // Should test existing
    });

    it('should return true if credentials exist', () => {
      providerC = new Provider('providerC');
      fs.mkdir(path.join(os.homedir(), '.another'));
      fs.writeFileSync(credentialsPathC, 'my credentials');
      expect(providerC.credsExist()).to.be.true;
    });
  });

  describe('getCredsPath()', () => {
    it('should return the correct path', () => {
      expect(providerC.getCredsPath()).to.equal(credentialsPathC);
    });

    it('should return correct path, also when the file does not exist',
      () => {
        expect(providerA.getCredsPath()).to.equal(credentialsPathA);
      });
  });

  describe('requiresSsh', () => {
    it('should return true if SSH keys are required for SSH', () => {
      expect(providerA.requiresSsh).to.be.true;
    });

    it('should return false if SSH keys are not required for SSH', () => {
      expect(providerB.requiresSsh).to.be.false;
    });
  });
});
