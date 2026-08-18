package thinkingoverrides

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type mongoOverrideDocument struct {
	ID           string    `bson:"_id"`
	ProviderName string    `bson:"provider_name,omitempty"`
	Model        string    `bson:"model,omitempty"`
	Mode         Mode      `bson:"mode"`
	Thinking     bool      `bson:"thinking"`
	Effort       string    `bson:"effort,omitempty"`
	CreatedAt    time.Time `bson:"created_at"`
	UpdatedAt    time.Time `bson:"updated_at"`
}

type mongoOverrideIDFilter struct {
	ID string `bson:"_id"`
}

// MongoDBStore stores model thinking overrides in MongoDB.
type MongoDBStore struct {
	collection *mongo.Collection
}

// NewMongoDBStore creates collection indexes if needed.
func NewMongoDBStore(database *mongo.Database) (*MongoDBStore, error) {
	if database == nil {
		return nil, fmt.Errorf("database is required")
	}
	coll := database.Collection("model_thinking_overrides")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	indexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "provider_name", Value: 1}}},
		{Keys: bson.D{{Key: "model", Value: 1}}},
		{Keys: bson.D{{Key: "updated_at", Value: -1}}},
	}
	if _, err := coll.Indexes().CreateMany(ctx, indexes); err != nil {
		return nil, fmt.Errorf("create model_thinking_overrides indexes: %w", err)
	}
	return &MongoDBStore{collection: coll}, nil
}

func (s *MongoDBStore) List(ctx context.Context) ([]Override, error) {
	cursor, err := s.collection.Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("list model thinking overrides: %w", err)
	}
	defer cursor.Close(ctx)

	result := make([]Override, 0, 8)
	for cursor.Next(ctx) {
		var doc mongoOverrideDocument
		if err := cursor.Decode(&doc); err != nil {
			return nil, fmt.Errorf("decode model thinking override: %w", err)
		}
		result = append(result, overrideFromMongo(doc))
	}
	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("iterate model thinking overrides: %w", err)
	}
	return result, nil
}

func (s *MongoDBStore) Upsert(ctx context.Context, override Override) error {
	override, err := normalizeStoredOverride(override)
	if err != nil {
		return err
	}
	if err := override.Validate(); err != nil {
		return err
	}

	now := time.Now().UTC()
	if override.CreatedAt.IsZero() {
		override.CreatedAt = now
	}
	override.UpdatedAt = now

	doc := mongoOverrideDocument{
		ID:           override.Selector,
		ProviderName: override.ProviderName,
		Model:        override.Model,
		Mode:         override.Mode,
		Thinking:     override.Thinking,
		Effort:       override.Effort,
		CreatedAt:    override.CreatedAt,
		UpdatedAt:    override.UpdatedAt,
	}
	if _, err := s.collection.UpdateOne(
		ctx,
		mongoOverrideIDFilter{ID: override.Selector},
		bson.M{"$set": doc},
		options.UpdateOne().SetUpsert(true),
	); err != nil {
		return fmt.Errorf("upsert model thinking override: %w", err)
	}
	return nil
}

func (s *MongoDBStore) Delete(ctx context.Context, selector string) error {
	res, err := s.collection.DeleteOne(ctx, mongoOverrideIDFilter{ID: selector})
	if err != nil {
		return fmt.Errorf("delete model thinking override: %w", err)
	}
	if res.DeletedCount == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *MongoDBStore) Close() error {
	return nil
}

func overrideFromMongo(doc mongoOverrideDocument) Override {
	return Override{
		Selector:     doc.ID,
		ProviderName: doc.ProviderName,
		Model:        doc.Model,
		Mode:         doc.Mode,
		Thinking:     doc.Thinking,
		Effort:       doc.Effort,
		CreatedAt:    doc.CreatedAt,
		UpdatedAt:    doc.UpdatedAt,
	}
}
