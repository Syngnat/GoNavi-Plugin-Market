package shell

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Client is the database surface the JS runtime uses. Keeping it narrow lets
// the shell tests inject a fake deployment instead of connecting to MongoDB.
type Client interface {
	Ping(ctx context.Context) error
	ListDatabases(ctx context.Context) ([]DatabaseInfo, error)
	Database(name string) Database
	Disconnect(ctx context.Context) error
}

// Database is one Mongo database handle.
type Database interface {
	Name() string
	Collection(name string) Collection
	CollectionNames(ctx context.Context) ([]string, error)
	RunCommand(ctx context.Context, command any) (bson.D, error)
}

// Collection is one Mongo collection handle. Read methods return documents in
// BSON order and report truncation instead of silently cutting rows.
type Collection interface {
	Find(ctx context.Context, filter, projection any, limit int64) ([]bson.D, bool, error)
	FindOne(ctx context.Context, filter, projection any) (bson.D, bool, error)
	CountDocuments(ctx context.Context, filter any) (int64, error)
	Aggregate(ctx context.Context, pipeline any, limit int64) ([]bson.D, bool, error)
	InsertOne(ctx context.Context, document any) (any, error)
	UpdateOne(ctx context.Context, filter, update any, upsert bool) (int64, int64, error)
	DeleteOne(ctx context.Context, filter any) (int64, error)
}

// DatabaseInfo is one entry of the database list returned by connect.
type DatabaseInfo struct {
	Name       string `json:"name"`
	SizeOnDisk int64  `json:"sizeOnDisk"`
	Empty      bool   `json:"empty"`
}

type clientFactory func(ConnectionConfig) (Client, error)

// NewMongoClient builds a real driver client. It never stores the config, so
// credentials live only for the duration of the call.
func NewMongoClient(config ConnectionConfig) (Client, error) {
	uri, err := config.MongoURI()
	if err != nil {
		return nil, err
	}
	clientOptions := options.Client().
		ApplyURI(uri).
		SetAppName("GoNavi MongoShell").
		SetConnectTimeout(connectTimeout).
		SetServerSelectionTimeout(connectTimeout)
	tlsConfig, err := config.TLSConfig()
	if err != nil {
		return nil, err
	}
	if tlsConfig != nil {
		clientOptions.SetTLSConfig(tlsConfig)
	}
	client, err := mongo.Connect(clientOptions)
	if err != nil {
		return nil, wrapCodedError(codeConnectionFailed, "cannot create the MongoDB client", err)
	}
	return &mongoClient{client: client}, nil
}

type mongoClient struct {
	client *mongo.Client
}

func (c *mongoClient) Ping(ctx context.Context) error {
	return c.client.Ping(ctx, nil)
}

func (c *mongoClient) ListDatabases(ctx context.Context) ([]DatabaseInfo, error) {
	result, err := c.client.ListDatabases(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	databases := make([]DatabaseInfo, 0, len(result.Databases))
	for _, database := range result.Databases {
		databases = append(databases, DatabaseInfo{
			Name:       database.Name,
			SizeOnDisk: database.SizeOnDisk,
			Empty:      database.Empty,
		})
	}
	return databases, nil
}

func (c *mongoClient) Database(name string) Database {
	return &mongoDatabase{database: c.client.Database(name)}
}

func (c *mongoClient) Disconnect(ctx context.Context) error {
	return c.client.Disconnect(ctx)
}

type mongoDatabase struct {
	database *mongo.Database
}

func (d *mongoDatabase) Name() string { return d.database.Name() }

func (d *mongoDatabase) Collection(name string) Collection {
	return &mongoCollection{collection: d.database.Collection(name)}
}

func (d *mongoDatabase) CollectionNames(ctx context.Context) ([]string, error) {
	return d.database.ListCollectionNames(ctx, bson.M{})
}

func (d *mongoDatabase) RunCommand(ctx context.Context, command any) (bson.D, error) {
	var result bson.D
	if err := d.database.RunCommand(ctx, command).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

type mongoCollection struct {
	collection *mongo.Collection
}

func (c *mongoCollection) Find(ctx context.Context, filter, projection any, limit int64) ([]bson.D, bool, error) {
	findOptions := options.Find().SetLimit(limit + 1)
	if projection != nil {
		findOptions.SetProjection(projection)
	}
	cursor, err := c.collection.Find(ctx, filterOrEmpty(filter), findOptions)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = cursor.Close(ctx) }()
	return decodeCursor(ctx, cursor, limit)
}

func (c *mongoCollection) FindOne(ctx context.Context, filter, projection any) (bson.D, bool, error) {
	findOptions := options.FindOne()
	if projection != nil {
		findOptions.SetProjection(projection)
	}
	var document bson.D
	err := c.collection.FindOne(ctx, filterOrEmpty(filter), findOptions).Decode(&document)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return document, true, nil
}

func (c *mongoCollection) CountDocuments(ctx context.Context, filter any) (int64, error) {
	return c.collection.CountDocuments(ctx, filterOrEmpty(filter))
}

func (c *mongoCollection) Aggregate(ctx context.Context, pipeline any, limit int64) ([]bson.D, bool, error) {
	cursor, err := c.collection.Aggregate(ctx, filterOrEmpty(pipeline), options.Aggregate().SetBatchSize(int32(minInt64(limit+1, 1000))))
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = cursor.Close(ctx) }()
	return decodeCursor(ctx, cursor, limit)
}

func (c *mongoCollection) InsertOne(ctx context.Context, document any) (any, error) {
	result, err := c.collection.InsertOne(ctx, document)
	if err != nil {
		return nil, err
	}
	return result.InsertedID, nil
}

func (c *mongoCollection) UpdateOne(ctx context.Context, filter, update any, upsert bool) (int64, int64, error) {
	result, err := c.collection.UpdateOne(ctx, filterOrEmpty(filter), update, options.UpdateOne().SetUpsert(upsert))
	if err != nil {
		return 0, 0, err
	}
	return result.MatchedCount, result.ModifiedCount, nil
}

func (c *mongoCollection) DeleteOne(ctx context.Context, filter any) (int64, error) {
	result, err := c.collection.DeleteOne(ctx, filterOrEmpty(filter))
	if err != nil {
		return 0, err
	}
	return result.DeletedCount, nil
}

func decodeCursor(ctx context.Context, cursor *mongo.Cursor, limit int64) ([]bson.D, bool, error) {
	documents := make([]bson.D, 0, limit)
	truncated := false
	for cursor.Next(ctx) {
		if int64(len(documents)) >= limit {
			truncated = true
			break
		}
		var document bson.D
		if err := cursor.Decode(&document); err != nil {
			return nil, false, err
		}
		documents = append(documents, document)
	}
	if err := cursor.Err(); err != nil {
		return nil, false, err
	}
	return documents, truncated, nil
}

func filterOrEmpty(value any) any {
	if value == nil {
		return bson.D{}
	}
	return value
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

// pingTimeout is used by connect/ping so a dead server fails within the
// documented ten-second budget even when the host request budget is larger.
const pingTimeout = connectTimeout

func pingContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, pingTimeout)
}
