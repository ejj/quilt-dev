const template = require('./package_template.json');
const initializer = require('../initializer/package.json');

template.dependencies = initializer.dependencies;
console.log(JSON.stringify(template, null, 2));
