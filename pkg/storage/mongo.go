//go:build experimental_mongo

// Experimental: This driver is experimental and subject to change.
package storage

import (
	"github.com/hangry-coder/bffx/pkg/manifest"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	bffx_errors "github.com/hangry-coder/bffx/pkg/errors"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type MongoStore struct {
	client *mongo.Client
	db     *mongo.Database
}

func NewMongoStore(uri, dbName string) (*MongoStore, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("mongo connect: %w", err)
	}

	err = client.Ping(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("mongo ping: %w", err)
	}

	return &MongoStore{
		client: client,
		db:     client.Database(dbName),
	}, nil
}

func (s *MongoStore) Create(ctx context.Context, resource string, payload map[string]any) (map[string]any, error) {
	collection := s.db.Collection(strings.ToLower(resource))
	
	if payload == nil {
		payload = make(map[string]any)
	}
	
	// Auto-Timestamps
	now := time.Now().UTC().Format(time.RFC3339)
	payload["created_at"] = now
	payload["updated_at"] = now
	
	_, err := collection.InsertOne(ctx, payload)
	if err != nil {
		return nil, err
	}
	
	return payload, nil
}

func (s *MongoStore) List(ctx context.Context, resource string, limit, offset int) ([]map[string]any, error) {
	collection := s.db.Collection(strings.ToLower(resource))
	
	opts := options.Find().SetLimit(int64(limit)).SetSkip(int64(offset))
	cursor, err := collection.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	
	var results []map[string]any
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	
	return results, nil
}

func (s *MongoStore) Get(ctx context.Context, resource, id string) (map[string]any, error) {
	collection := s.db.Collection(strings.ToLower(resource))
	
	var result map[string]any
	err := collection.FindOne(ctx, bson.M{"id": id}).Decode(&result)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, bffx_errors.ErrNotFound
		}
		return nil, err
	}
	
	return result, nil
}

func (s *MongoStore) Update(ctx context.Context, resource, id string, payload map[string]any) (map[string]any, error) {
	collection := s.db.Collection(strings.ToLower(resource))
	
	// Prevent overwriting internal fields
	delete(payload, "id")
	delete(payload, "created_at")
	
	payload["updated_at"] = time.Now().UTC().Format(time.RFC3339)
	
	res, err := collection.UpdateOne(ctx, bson.M{"id": id}, bson.M{"$set": payload})
	if err != nil {
		return nil, err
	}
	if res.MatchedCount == 0 {
		return nil, bffx_errors.ErrNotFound
	}
	
	return s.Get(ctx, resource, id)
}

func (s *MongoStore) Delete(ctx context.Context, resource, id string) error {
	collection := s.db.Collection(strings.ToLower(resource))
	
	res, err := collection.DeleteOne(ctx, bson.M{"id": id})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return bffx_errors.ErrNotFound
	}
	return nil
}

func (s *MongoStore) GetByField(ctx context.Context, resource, field, value string) (map[string]any, error) {
	collection := s.db.Collection(strings.ToLower(resource))
	var result map[string]any
	err := collection.FindOne(ctx, bson.M{field: value}).Decode(&result)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, bffx_errors.ErrNotFound
		}
		return nil, err
	}
	return result, nil
}

func (s *MongoStore) ListByOwner(ctx context.Context, resource, ownerID string, limit, offset int) ([]map[string]any, error) {
	collection := s.db.Collection(strings.ToLower(resource))
	opts := options.Find().SetLimit(int64(limit)).SetSkip(int64(offset))
	cursor, err := collection.Find(ctx, bson.M{"created_by": ownerID}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var results []map[string]any
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	return results, nil
}

func (s *MongoStore) Reconcile(ctx context.Context, reg *manifest.Registry) ([]Change, error) {
	// TODO: Implement Mongo collection & index reconciliation (Week 9)
	return nil, fmt.Errorf("%w: Mongo reconciliation not implemented", bffx_errors.ErrNotImplemented)
}

func (s *MongoStore) GetChildren(ctx context.Context, resource, id string) ([]map[string]any, error) {
	return nil, fmt.Errorf("%w: GetChildren not implemented for Mongo", bffx_errors.ErrNotImplemented)
}
func (s *MongoStore) GetAncestors(ctx context.Context, resource, id string) ([]map[string]any, error) {
	return nil, fmt.Errorf("%w: GetAncestors not implemented for Mongo", bffx_errors.ErrNotImplemented)
}

func (s *MongoStore) Query(ctx context.Context, resource string) QueryBuilder {
	return &MongoQueryBuilder{
		store:      s,
		collection: s.db.Collection(strings.ToLower(resource)),
		filter:     bson.M{},
		limit:      100,
		ctx:        ctx,
	}
}

type MongoQueryBuilder struct {
	store      *MongoStore
	collection *mongo.Collection
	filter     bson.M
	limit      int
	offset     int
	sort       bson.D
	ctx        context.Context
}

func (q *MongoQueryBuilder) Where(field, op string, value any) QueryBuilder {
	opLower := strings.ToLower(op)
	switch opLower {
	case "eq", "=":
		q.filter[field] = value
	case "gt", ">":
		q.filter[field] = bson.M{"$gt": value}
	case "lt", "<":
		q.filter[field] = bson.M{"$lt": value}
	case "gte", ">=":
		q.filter[field] = bson.M{"$gte": value}
	case "lte", "<=":
		q.filter[field] = bson.M{"$lte": value}
	case "like":
		if strVal, ok := value.(string); ok {
			strVal = strings.Trim(strVal, "%")
			q.filter[field] = bson.M{"$regex": strVal, "$options": "i"}
		} else {
			q.filter[field] = value
		}
	case "in":
		q.filter[field] = bson.M{"$in": value}
	}
	return q
}

func (q *MongoQueryBuilder) WhereIn(field string, values []any) QueryBuilder {
	q.filter[field] = bson.M{"$in": values}
	return q
}

func (q *MongoQueryBuilder) OrderBy(field string, desc bool) QueryBuilder {
	direction := 1
	if desc { direction = -1 }
	q.sort = append(q.sort, bson.E{Key: field, Value: direction})
	return q
}

func (q *MongoQueryBuilder) Limit(n int) QueryBuilder { q.limit = n; return q }
func (q *MongoQueryBuilder) Offset(n int) QueryBuilder { q.offset = n; return q }
func (q *MongoQueryBuilder) Execute(ctx context.Context) ([]map[string]any, error) {
	opts := options.Find().SetLimit(int64(q.limit)).SetSkip(int64(q.offset))
	if len(q.sort) > 0 {
		opts.SetSort(q.sort)
	}
	cursor, err := q.collection.Find(ctx, q.filter, opts)
	if err != nil {
		return nil, err
	}
	var res []map[string]any
	if err := cursor.All(ctx, &res); err != nil {
		return nil, err
	}
	return res, nil
}

func (q *MongoQueryBuilder) Count(ctx context.Context) (int, error) {
	count, err := q.collection.CountDocuments(ctx, q.filter)
	if err != nil {
		return 0, err
	}
	return int(count), nil
}
