const path = require('path');
const os = require('os');

const quiltSshKeyLocationPrivate = path.join(os.homedir(), '.ssh', 'quilt');
const quiltSshKeyLocationPublic = path.join(os.homedir(), '.ssh', 'quilt.pub');

// Used in infrastructure template.
const namespace = 'namespace';
const provider = 'provider';

const size = 'size';
const ram = 'ram';
const cpu = 'cpu';
const preemptible = 'preemptible';
const region = 'region';

const sshKey = 'sshKey';

const masterCount = 'masterCount';
const workerCount = 'workerCount';


// Infrastructure.
const name = 'name';
const infraOverwrite = 'infraOverwrite';

// Provider.
const providerUseExistingKey = 'useExistingKey';
const providerCredsPath = 'credsPath';
const credsConfirmOverwrite = 'confirmOverwrite';

// SSH.
const sshUseExistingKey = 'Use existing SSH key';
const sshGenerateKey = 'Generate new SSH key pair';
const skip = 'Skip';
const sshKeyOption = 'sshKeyOption';
const sshKeyPath = 'sshKeyPath';

// Size.
const other = 'other';
const sizeType = 'sizeType';
const instanceTypeSize = 'Instance type';
const ramCpuSize = 'RAM & CPU';

const inputCredsPath = 'credsPath';

module.exports = {
  quiltSshKeyLocationPrivate,
  quiltSshKeyLocationPublic,

  name,
  namespace,
  infraOverwrite,
  provider,
  providerUseExistingKey,
  providerCredsPath,
  credsConfirmOverwrite,
  sshKey,
  sshUseExistingKey,
  sshGenerateKey,
  skip,
  sshKeyOption,
  sshKeyPath,
  other,
  sizeType,
  size,
  instanceTypeSize,
  ramCpuSize,
  ram,
  cpu,
  preemptible,
  region,
  masterCount,
  workerCount,
  inputCredsPath,
};
