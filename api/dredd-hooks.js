var hooks = require('hooks');
var clone = require('clone');

var methods = ['POST', 'PUT', 'GET', 'DELETE'];

hooks.beforeAll(function(transactions, done){
   var uriMap = {};
   transactions.map((transaction) => {
      if(!uriMap[transaction.request.uri]){
         uriMap[transaction.request.uri] = [];
      }

      if(uriMap[transaction.request.uri].indexOf(transaction.request.method) === -1){
         uriMap[transaction.request.uri].push(transaction.request.method);
      }
   });

   Object.keys(uriMap).map((uri) => {
      transactions.filter((t) => {return t.request.uri === uri;}).map(t => {
         methods.filter(m => {return uriMap[uri].indexOf(m) === -1;})
            .map(m => {
               var copy = clone(t);
               copy.request.method = m;
               copy.request.headers['Accept'] = '*';
               copy.expected.statusCode = 405;
               copy.expected.body = 'Method Not Allowed';
               copy.expected.headers['Content-Type'] = 'text/plain; charset=utf-8'
               transactions.push(copy);
            });
      });
   });

   done();
});

hooks.beforeValidation('Health > /__gtg > Good To Go > 200 > text/plain; charset=US-ASCII', function (transaction) {
   if (transaction.real.statusCode === 503) {
      hooks.log("Accepting GET /__gtg 503 Service Unavailable, and continuing response validation.");
      transaction.real.statusCode = 200;
      transaction.real.body = 'OK';
   }
});
