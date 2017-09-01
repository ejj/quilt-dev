const path = require('path');
const os = require('os');
const fs = require('fs');

const inquirer = require('inquirer');
const consts = require('./constants');
const util = require('./init-util');
const Provider = require('./provider');


/**
  * Return a list of all provider names as listed in providers.json.
  *
  * @return {string[]} A list of provider names.
  */
function allProviders() {
  const providerFile = path.join(__dirname, 'providers.json');
  const providerInfo = JSON.parse(fs.readFileSync(providerFile, 'utf8'));
  return Object.keys(providerInfo);
}
/**
  * Check if the Quilt SSH keys exists.
  *
  * @return {boolean} True iff both the private and public Quilt SSH keys exist.
  */
function quiltSshKeyExists() {
  return (fs.existsSync(consts.quiltSshKeyLocationPublic) &&
      fs.existsSync(consts.quiltSshKeyLocationPrivate));
}

/**
  * Throw an error if the input string does not contain a number.
  *
  * @param {string} input The string to check.
  * @return {void}
  */
function isNumber(input) {
  if (!(/^\d+$/.test(input))) {
    throw new Error('Please provide a number');
  }
  return true;
}

/**
  * Get user friendly descriptions for the instance types listed for the
  * chosen provider in providers.json.
  * The output map would have entries like {'small (m3.medium)': 'm3.medium'}
  *
  * @param {Provider} provider The chosen Provider.
  * @return {Object.<string, string>} A map from the size description to the
  *   corresponding size.
  */
function getSizeDescriptions(provider) {
  const sizeDescriptions = {};
  const sizes = provider.getSizes();
  const sizeCategories = Object.keys(sizes);

  sizeCategories.forEach((category) => {
    const size = sizes[category];
    const description = `${category} (${size})`;
    sizeDescriptions[description] = size;
  });
  return sizeDescriptions;
}

// User prompts.

/**
  * Prompt the user for a valid infrastructure name. Users will use this name in
  * their blueprints to get the right infrastructure.
  *
  * @return {Promise} A promise that contains the user's answers.
  */
function infraNamePrompt() {
  const questions = [
    {
      type: 'input',
      name: consts.name,
      message: 'Pick a name for your infrastructure:',
      default: 'default',
    },
    {
      type: 'confirm',
      name: consts.infraOverwrite,
      message(answers) {
        return `"${answers[consts.name]}" already exists. Overwrite?`;
      },
      default: false,
      when(answers) {
        return fs.existsSync(util.infraPath(answers[consts.name]));
      },
    },
  ];

  /**
    * Repeatedly prompt for a name until we get a valid answer.
    *
    * @return {Promise} A promise that contains the user's answers
    */
  function confirmIfOverwrite() {
    return inquirer.prompt(questions).then((answers) => {
      if (answers[consts.infraOverwrite] === false) {
        return confirmIfOverwrite();
      }
      return answers;
    });
  }
  return confirmIfOverwrite();
}

/**
  * Prompt the user for for their desired provider.
  *
  * @return {Promise} A promise that contains the user's answer.
  */
function getProviderPrompt() {
  const questions = [
    {
      type: 'list',
      name: consts.provider,
      message: 'Choose a provider:',
      choices() {
        return allProviders();
      },
    },
  ];

  return inquirer.prompt(questions);
}

/**
  * Ask the user about SSH keys for the infrastructure.
  *
  * @param {Provider} provider The chosen provider.
  * @return {Promise} A promise that contains the user's answers.
  */
function sshKeyPrompts(provider) {
  if (!provider.requiresSsh) return { [consts.sshKeyOption]: consts.skip };

  let choices = [consts.sshUseExistingKey, consts.skip];
  if (!quiltSshKeyExists()) {
    choices = [consts.sshGenerateKey].concat(choices);
  }

  const questions = [
    {
      type: 'list',
      name: consts.sshKeyOption,
      message: 'Choose an SSH key to log into VMs and containers:',
      choices,
    },
    {
      type: 'input',
      name: consts.sshKeyPath,
      message: 'Path to public SSH key:',

      when(answers) {
        return answers[consts.sshKeyOption] === consts.sshUseExistingKey;
      },

      default() {
        if (quiltSshKeyExists()) return consts.quiltSshKeyLocationPublic;
        return path.join(os.homedir(), '.ssh', 'id_rsa.pub');
      },

      validate(input) {
        if (fs.existsSync(input)) return true;
        return `Oops, no file called ${input}`;
      },
    },
  ];

  return inquirer.prompt(questions);
}

/**
  * Ask the user about provider credentials.
  *
  * @param {Provider} provider The provider to set up credentials for.
  * @return {Promise} A promise that contains the user's answers.
  */
function credentialsPrompts(provider) {
  if (!provider.requiresCreds()) {
    return new Promise((resolve) => {
      resolve({});
    });
  }

  const keyExists = provider.credsExist();
  const questions = [
    {
      type: 'confirm',
      name: consts.providerUseExistingKey,
      message: `Use existing keys for ${provider.getName()}` +
        ` (${provider.getCredsPath()})?`,
      default: true,
      when() {
        return keyExists;
      },
    },
    {
      type: 'confirm',
      name: consts.credsConfirmOverwrite,
      message: 'This will overwrite the existing credentials. Continue?' +
        ' (If no, use existing credentials)',
      default: false,
      when(answers) {
        return answers[consts.providerUseExistingKey] === false;
      },
    },
  ];

  const keys = provider.getCredsKeys();
  const keyNames = Object.keys(keys);

  keyNames.forEach((keyName) => {
    questions.push(
      {
        type: 'input',
        name: keyName,
        message: `${keys[keyName]}:`,
        // Ask this questions for all credential inputs that are not paths.
        // I.e. keys given exactly as they should appear in the credential file.
        when(answers) {
          return (keyName !== consts.inputCredsPath &&
          (!keyExists || answers[consts.credsConfirmOverwrite]));
        },
        filter(input) {
          return input.trim();
        },
      },
      {
        type: 'input',
        name: keyName,
        message: `${keys[keyName]}:`,

        // Ask this question if the credentials should be given as a file path.
        // E.g. the path to the GCE project ID file.
        when(answers) {
          return keyName === consts.inputCredsPath &&
            (!keyExists || answers[consts.credsConfirmOverwrite]);
        },

        validate(input) {
          if (fs.existsSync(input)) return true;
          return `Oops, no file called "${input}".`;
        },

        filter(input) {
          return path.resolve(input);
        },
      });
  });
  return inquirer.prompt(questions);
}

/**
  * Ask the user for machine configuration, such as size and region.
  *
  * @param {Provider} provider The provider chosen for this infrastructure.
  * @return {Promise} A promise that contains the user's answers.
  */
function machineConfigPrompts(provider) {
  const sizeChoices = getSizeDescriptions(provider);
  sizeChoices[consts.other] = consts.other;
  const questions = [
    {
      type: 'confirm',
      name: consts.preemptible,
      message: 'Do you want to run preemptible instances?',
      default: false,
      when() {
        return provider.hasPreemptible;
      },
    },
    {
      type: 'list',
      name: consts.region,
      message: 'Which region do you want to deploy in?',
      choices: provider.getRegions(),
      when() {
        return provider.regions !== undefined;
      },
    },
    {
      type: 'list',
      name: consts.sizeType,
      message: 'How do you want to specify the size of your machine?',
      choices: [consts.instanceTypeSize, consts.ramCpuSize],
      when() {
        // If there are no instance types, just ask for RAM and CPU.
        return provider.sizes !== undefined;
      },
    },
    {
      type: 'list',
      name: consts.size,
      message: 'Choose an instance:',
      choices: Object.keys(sizeChoices),
      filter(input) {
        // The user's input will be something like 'small (m3.medium)', so we
        // translate that into m3.medium, which we can put in the blueprint.
        return sizeChoices[input];
      },
      when(answers) {
        return answers[consts.sizeType] === consts.instanceTypeSize;
      },
    },
    {
      type: 'input',
      name: consts.size,
      message: 'Which other instance type?',
      validate(input) {
        if (input !== '') return true;
        return 'Please provide an instance type';
      },
      when(answers) {
        return answers[consts.size] === consts.other;
      },
    },
    {
      type: 'input',
      name: consts.cpu,
      message: 'How many CPUs do you want?',
      default: 1,
      validate(input) {
        return isNumber(input);
      },
      when(answers) {
        return answers[consts.sizeType] !== consts.instanceTypeSize;
      },
    },
    {
      type: 'input',
      name: 'ram',
      message: 'How many GiB of RAM do you want?',
      default: 2,
      validate(input) {
        return isNumber(input);
      },
      when(answers) {
        return answers[consts.sizeType] !== consts.instanceTypeSize;
      },
    },
  ];

  return inquirer.prompt(questions);
}

/**
  * Ask for the desired number of machines.
  *
  * @return {Promise} A promise that contains the user's answers.
  */
function machineCountPrompts() {
  const questions = [
    {
      type: 'input',
      name: consts.masterCount,
      message: 'How many master VMs?',
      default: 1,
      validate(input) {
        return isNumber(input);
      },
    },
    {
      type: 'input',
      name: consts.workerCount,
      message: 'How many worker VMs?',
      default: 1,
      validate(input) {
        return isNumber(input);
      },
    },
  ];

  return inquirer.prompt(questions);
}

/**
 * Prompt the user to get the information needed to create the new
 * infrastructure.
 *
 * @return {Promise} A promise that contains the user's answers.
 */
function promptUser() {
  const answers = {};
  return infraNamePrompt()
    .then((infraAnswers) => {
      Object.assign(answers, infraAnswers);
      return getProviderPrompt();
    })


    .then((providerAnswer) => {
      Object.assign(answers, providerAnswer);
      const provider = new Provider(answers[consts.provider]);
      return credentialsPrompts(provider)

        .then((keyAnswers) => {
          Object.assign(answers, keyAnswers);
          return machineConfigPrompts(provider);
        })

        .then((providerAnswers) => {
          Object.assign(answers, providerAnswers);
          return sshKeyPrompts(provider);
        })

        .then((sshKeyAnswers) => {
          Object.assign(answers, sshKeyAnswers);
          return machineCountPrompts();
        })

        .then((machineAnswers) => {
          Object.assign(answers, machineAnswers);
          return { provider, answers };
        });
    });
}

module.exports = { promptUser };
