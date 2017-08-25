#!/usr/bin/env node

const path = require('path');
const os = require('os');
const execSync = require('child_process').execSync;
const exit = require('process').exit;
// Assign imports to variables rather than consts so the modules can
// be mocked out in tests.
let fs = require('fs');
let fsExtra = require('fs-extra');
let log = console.log;

const handlebars = require('handlebars');

const prompter = require('./prompter');
const consts = require('./constants');

const templateDir = path.join(__dirname, 'templates');
const infraTemplateFile = path.join(templateDir, 'inf_template');
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

/**
  * Creates a file from the given template and context, and writes the result to
  * a file.
  *
  * @param {string} templateLocation The path to the template file.
  * @param {Object.<string, string>} context The context to apply to the
  *   template.
  * @param {string} destination The path to write the result to.
  * @return {void}
  */
function createFileFromTemplate(templateLocation, context, destination) {
  if (!fs.existsSync(templateLocation)) {
    throw new Error(`failed to create file at ${destination}.` +
      ` No template in ${templateLocation}.`);
  }
  const rawTemplate = fs.readFileSync(templateLocation, 'utf8');
  const template = handlebars.compile(rawTemplate);
  fs.writeFileSync(destination, template(context));
}

/**
  * Given a file path, recursively create all directories in the path.
  * This function assumes that the passed in path is to a file, not a directory.
  *
  * @param {string} filePath The complete file path for which to create
  *  directories.
  * @return {void}
  */
function makeDirPath(filePath) {
  const lastSlash = filePath.lastIndexOf('/');
  const directories = filePath.slice(0, lastSlash);
  const fullPath = path.join(os.homedir(), directories);

  fsExtra.mkdirp(fullPath, (err) => {
    if (err) {
      throw new Error(
        `failed to create credentials file in ${fullPath}: ${err}`);
    }
  });
}

/**
  * Create an SSH key pair in the default Quilt SSH key location. Will throw
  * an error if the key creation fails.
  *
  * @return {void}
  */
function sshGenerateKeyPair() {
  try {
    execSync(
      `ssh-keygen -t rsa -b 2048 -f ${consts.quiltSshKeyLocationPrivate}` +
      ` -N '' -C ''`,
      {stdio: ['ignore', 'ignore', 'pipe']});
  } catch (err) {
    throw new Error('failed to create SSH key: ' +
      `${err.stderr.toString().trim()}`);
  }
}

/**
  * Create new credentials file if needed.
  *
  * @param {Provider} provider The chosen provider.
  * @param {Promise} answers A promise that contains the user's answers.
  * @return {void}
  */
function writeProviderCreds(provider, answers) {
  if (!provider.requiresCreds() ||
      answers[consts.providerUseExistingKey] ||
      answers[consts.credsConfirmOverwrite] === false) {
    return;
  }

  const credentialsDest = provider.getCredsPath();
  makeDirPath(provider.getCredsPath());

  if (provider.credsTemplate !== undefined) {
    const templateFile = path.join(templateDir, provider.getCredsTemplate());
    createFileFromTemplate(templateFile, answers, credentialsDest);
  } else {
    // We were given a file path instead of the keys themselves.
    const credentialsSrc = answers[consts.inputCredsPath];
    fsExtra.copy(credentialsSrc, credentialsDest, (err) => {
      if (err) throw err;
    });
  }

  log(`Wrote credentials to ${credentialsDest}`);
}

/**
  * Retrieve the correct SSH key based on the user's input. This can be no key,
  * a newly Quilt-generated SSH key, or an existing SSH key at user-given path.
  *
  * @param {Promise} answers promise that contains the user's answers.
  * @return {string} The relevant SSH key.
  */
function getSshKey(answers) {
  let keyPath;

  switch (answers[consts.sshKeyOption]) {
    case consts.skip:
      return '';

    case consts.sshGenerateKey:
      sshGenerateKeyPair();
      log(`Created SSH key pair in ${consts.quiltSshKeyLocationPrivate}`);
      keyPath = consts.quiltSshKeyLocationPublic;
      break;

    case consts.sshUseExistingKey:
      keyPath = answers[consts.sshKeyPath];
      break;

    default:
      throw new Error('Unrecognized ssh key option: ' +
        `${answers[consts.sshKeyOption]}`);
  }
  return fs.readFileSync(keyPath, 'utf8').trim();
}

/** Create the infrastructure file, SSH keys, and credentials based on the
  * user's input.
  *
  * @param {Provider} provider The chosen provider.
  * @param {Promise} answers A promise that contains the user's answers.
  * @return {void}
  */
function processAnswers(provider, answers) {
  fsExtra.mkdirp(infraDirectory, (err) => {
    if (err) throw new Error(`Failed to create ${infraDirectory}: ${err}`);
  });

  writeProviderCreds(provider, answers);

  const config = {
    [consts.provider]: answers[consts.provider],
    [consts.size]: answers[consts.size],
    [consts.ram]: answers[consts.ram],
    [consts.cpu]: answers[consts.cpu],
    [consts.preemptible]: answers[consts.preemptible] || false,
    [consts.region]: answers[consts.region],
    [consts.sshKey]: getSshKey(answers),
    [consts.masterCount]: answers[consts.masterCount],
    [consts.workerCount]: answers[consts.workerCount],
  };

  const outputPath = infraPath(answers[consts.name]);
  createFileFromTemplate(infraTemplateFile, config, outputPath);
  log(`Created infrastructure in ${outputPath}`);
}

/**
  * Get user input, and process the answers.
  * @return {void}
  */
function run() {
  prompter.promptUser()
  .then((result) => {
    processAnswers(result.provider, result.answers);
  })
  .catch((error) => {
    console.warn(`Oops, something went wrong: ${error.message}`);
    exit(1);
  });
}

if (require.main === module) {
  console.log(`---------------------------------------------------------
|   See docs at http://docs.quilt.io/#getting-started   |
---------------------------------------------------------`);
  run();
}

module.exports = {
  createFileFromTemplate,
  writeProviderCreds,
  getSshKey,
  processAnswers,
};
