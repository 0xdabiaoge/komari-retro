const filter = require('./guest-locales.cjs');
module.exports = function(source) { return `module.exports = ${JSON.stringify(filter(JSON.parse(source)))};`; };
