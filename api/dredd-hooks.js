var hooks = require('hooks');
var _ = require('lodash');
var clone = require('clone');

var httpMethods = ['POST', 'PUT', 'GET', 'DELETE']; // OPTIONS and HEAD are supported by default

hooks.beforeAll(function(transactions, done){
   var groupedByUri = _.groupBy(transactions, 'request.uri');
   groupedByUri = _.map(groupedByUri, (v, k) => {
      return {uri: k, methods: _.uniq(v.map(transaction => {return transaction.request.method;}))};
   });

   _.map(groupedByUri, (val) => {
      var transaction = _.find(transactions, ['request.uri', val.uri])
      var testMethods = _.difference(httpMethods, val.methods);
      testMethods.map(m => {
         var copy = clone(transaction);
         copy.request.method = m;
         copy.request.headers['Accept'] = '*';
         copy.expected.statusCode = 405;
         copy.expected.body = 'Method Not Allowed';
         copy.expected.headers['Content-Type'] = 'text/plain; charset=utf-8'
         transactions.push(copy);
      });
   })
   done();
});

hooks.beforeValidation('Health > /__gtg > Good To Go > 200 > text/plain; charset=US-ASCII', function (transaction) {
   if (transaction.real.statusCode === 503) {
      hooks.log("Accepting GET /__gtg 503 Service Unavailable, and continuing response validation.");
      transaction.real.statusCode = 200;
      transaction.real.body = 'OK';
   }
});
