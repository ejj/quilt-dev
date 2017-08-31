const path = require('path');
const os = require('os');

// Both infraDirectory and infraPath are also defined in bindings.js.
// This code duplication is ugly, but it significantly simplifies packaging
// the `quilt init` code with the "@quilt/install" module.
const infraDirectory = path.join(os.homedir(), '.quilt', 'infra');

/**
  * Returns the absolute path to the infrastructure with the given name.
  *
  * @param {string} infraName The name of the infrastructure.
  * @return {string} The absolute path to the infrastructure file.
  */
function infraPath(infraName) {
  return path.join(infraDirectory, `${infraName}.js`);
}

module.exports = {
  infraDirectory,
  infraPath,
};
