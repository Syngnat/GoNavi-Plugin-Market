# MongoShell recipes

MongoShell opens a dedicated window for a saved MongoDB connection and runs
JavaScript inside the plugin's Go backend with goja. It is not a general-purpose
Node.js or MongoDB `mongosh` environment: scripts see only the helpers below and
are bounded by rows and time.

## Safety rules

- The launch context decides read-only mode. When `readOnly` is true, every
  write helper and every mutating `runCommand` is rejected before a request is
  sent; do not try to work around it.
- Bound every read. `find` and `aggregate` use `maxRows` (default 200, maximum
  2000); only the newest result table is returned to the window.
- A script has 30 seconds. Long-running server operations continue only until
  the driver context is cancelled.
- Never print credentials, connection strings or saved configuration. The shell
  never exposes the connection configuration to JavaScript.

## Script API

```js
print(...values)          // append one line to the output pane
use("databaseName")       // switch the current database, returns db

db.getName()
db.getCollectionNames()   // bounded collection list
db.getCollection("name")  // explicit collection handle
db.runCommand({ ... })    // read-only sessions reject mutating commands

const users = db.users     // dynamic collection access
users.find(filter, projection, limit) // array of documents, bounded
users.findOne(filter, projection)     // document or null
users.countDocuments(filter)
users.aggregate(pipeline)            // array of documents, bounded
users.insertOne(document)
users.updateOne(filter, update, {upsert})
users.deleteOne(filter)
```

`find` and `aggregate` return arrays of JSON-safe documents, so ordinary
JavaScript works:

```js
const rows = db.orders.find({status: "open"}, {amount: 1, customer: 1}, 50);
print("open orders:", rows.length);
rows.map((row) => row.customer);
```

Aggregation pipelines use the server's stage syntax:

```js
db.orders.aggregate([
  {$match: {status: "open"}},
  {$group: {_id: "$customer", total: {$sum: "$amount"}}},
  {$sort: {total: -1}},
  {$limit: 20}
]);
```

## Result envelope

Every command returns `{ok: true, data: ...}` or
`{ok: false, error: {code, message}}`. Stable failure codes include
`invalid_argument`, `not_connected`, `unsupported_connection`,
`unsupported_transport`, `connection_failed`, `query_failed`, `read_only`,
`script_error`, `syntax_error` and `timeout`.

## Known limitations

- SSH tunnels, HTTP proxies/tunnels and replica-user fallback are rejected in
  v1. Use a direct MongoDB connection with an explicit URI when needed.
- The window bridge accepts at most 1 MiB per response, so a single very large
  document can fail even when the row limit is respected. Project fields or
  aggregate before returning wide documents.
