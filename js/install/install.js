const os = require('os');
const fs = require('fs');

const platform = os.platform();
switch (platform) {
  case 'darwin':
    fs.symlinkSync('./quilt_darwin', './quilt');
    break;
  case 'linux':
    fs.symlinkSync('./quilt_linux', './quilt');
    break;
  default:
    throw new Error(`Unrecognized operating system ${platform}. Bailing.`);
}
