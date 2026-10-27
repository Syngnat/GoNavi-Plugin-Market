package shell

import (
	"context"
	"sort"
	"sync"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// fakeClient is a bounded in-memory stand-in for the driver client. The shell
// tests never open a network connection.
type fakeClient struct {
	mu           sync.Mutex
	databases    []DatabaseInfo
	byName       map[string]*fakeDatabase
	pingErr      error
	listErr      error
	pingCalls    int
	disconnected bool
}

func newFakeClient(databases ...string) *fakeClient {
	client := &fakeClient{byName: make(map[string]*fakeDatabase)}
	for _, name := range databases {
		client.databases = append(client.databases, DatabaseInfo{Name: name})
		client.byName[name] = newFakeDatabase(name)
	}
	return client
}

func (c *fakeClient) Ping(context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pingCalls++
	return c.pingErr
}

func (c *fakeClient) ListDatabases(context.Context) ([]DatabaseInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.listErr != nil {
		return nil, c.listErr
	}
	return append([]DatabaseInfo(nil), c.databases...), nil
}

func (c *fakeClient) Database(name string) Database {
	c.mu.Lock()
	defer c.mu.Unlock()
	database, exists := c.byName[name]
	if !exists {
		database = newFakeDatabase(name)
		c.byName[name] = database
	}
	return database
}

func (c *fakeClient) Disconnect(context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.disconnected = true
	return nil
}

type fakeDatabase struct {
	name         string
	collections  map[string]*fakeCollection
	names        []string
	command      bson.D
	commandErr   error
	lastCommand  any
	commandCalls int
}

func newFakeDatabase(name string) *fakeDatabase {
	return &fakeDatabase{name: name, collections: make(map[string]*fakeCollection)}
}

func (d *fakeDatabase) Name() string { return d.name }

func (d *fakeDatabase) Collection(name string) Collection {
	collection, exists := d.collections[name]
	if !exists {
		collection = &fakeCollection{}
		d.collections[name] = collection
	}
	return collection
}

func (d *fakeDatabase) CollectionNames(context.Context) ([]string, error) {
	if d.names != nil {
		return append([]string(nil), d.names...), nil
	}
	names := make([]string, 0, len(d.collections))
	for name := range d.collections {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func (d *fakeDatabase) RunCommand(_ context.Context, command any) (bson.D, error) {
	d.commandCalls++
	d.lastCommand = command
	if d.commandErr != nil {
		return nil, d.commandErr
	}
	return d.command, nil
}

type fakeCollection struct {
	docs    []bson.D
	findErr error
	block   bool

	findCalls      int
	findFilter     any
	findProjection any
	findLimit      int64

	aggregateDocs  []bson.D
	findOneDoc     bson.D
	findOneFound   bool
	findOneErr     error
	countValue     int64
	countErr       error
	countFilter    any
	inserted       []any
	insertID       any
	insertErr      error
	updateMatched  int64
	updateModified int64
	updateErr      error
	updateFilter   any
	updateDocument any
	updateUpsert   bool
	deletedCount   int64
	deleteErr      error
	deleteFilter   any
	aggregateErr   error
	aggregateLimit int64
}

func (c *fakeCollection) Find(ctx context.Context, filter, projection any, limit int64) ([]bson.D, bool, error) {
	c.findCalls++
	c.findFilter = filter
	c.findProjection = projection
	c.findLimit = limit
	if c.block {
		<-ctx.Done()
		return nil, false, ctx.Err()
	}
	if c.findErr != nil {
		return nil, false, c.findErr
	}
	documents := append([]bson.D(nil), c.docs...)
	truncated := int64(len(documents)) > limit
	if truncated {
		documents = documents[:limit]
	}
	return documents, truncated, nil
}

func (c *fakeCollection) FindOne(context.Context, any, any) (bson.D, bool, error) {
	if c.findOneErr != nil {
		return nil, false, c.findOneErr
	}
	return c.findOneDoc, c.findOneFound, nil
}

func (c *fakeCollection) CountDocuments(_ context.Context, filter any) (int64, error) {
	c.countFilter = filter
	if c.countErr != nil {
		return 0, c.countErr
	}
	return c.countValue, nil
}

func (c *fakeCollection) Aggregate(_ context.Context, pipeline any, limit int64) ([]bson.D, bool, error) {
	c.aggregateLimit = limit
	if c.aggregateErr != nil {
		return nil, false, c.aggregateErr
	}
	documents := append([]bson.D(nil), c.aggregateDocs...)
	truncated := int64(len(documents)) > limit
	if truncated {
		documents = documents[:limit]
	}
	return documents, truncated, nil
}

func (c *fakeCollection) InsertOne(_ context.Context, document any) (any, error) {
	if c.insertErr != nil {
		return nil, c.insertErr
	}
	c.inserted = append(c.inserted, document)
	if c.insertID != nil {
		return c.insertID, nil
	}
	return "generated-id", nil
}

func (c *fakeCollection) UpdateOne(_ context.Context, filter, update any, upsert bool) (int64, int64, error) {
	c.updateFilter = filter
	c.updateDocument = update
	c.updateUpsert = upsert
	if c.updateErr != nil {
		return 0, 0, c.updateErr
	}
	return c.updateMatched, c.updateModified, nil
}

func (c *fakeCollection) DeleteOne(_ context.Context, filter any) (int64, error) {
	c.deleteFilter = filter
	if c.deleteErr != nil {
		return 0, c.deleteErr
	}
	return c.deletedCount, nil
}
