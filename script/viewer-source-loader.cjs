module.exports = function(source) { return source.replaceAll('/api/', '/viewer-api/').replace(/(["'`])\/instance\//g, '$1/shared/instance/'); };
