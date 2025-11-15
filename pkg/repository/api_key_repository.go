package repository

import (
	"context"
	"time"

	"astral-backend/pkg/logger"
	"astral-backend/pkg/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// APIKeyRepository implements the APIKeyRepository interface for MongoDB
type APIKeyRepository struct {
	collection *mongo.Collection
	logger     *logger.Logger
}

// NewAPIKeyRepository creates a new API key repository
func NewAPIKeyRepository(db *mongo.Database, logger *logger.Logger) *APIKeyRepository {
	return &APIKeyRepository{
		collection: db.Collection("api_keys"),
		logger:     logger,
	}
}

// Create inserts a new API key into the database
func (r *APIKeyRepository) Create(ctx context.Context, key *models.APIKey) error {
	r.logger.Info("Creating API key in database",
		zap.String("key_name", key.Name),
		zap.Strings("scopes", key.Scopes),
	)

	// Set the ID if not already set
	if key.ID.IsZero() {
		key.ID = primitive.NewObjectID()
	}

	// Set timestamps if not set
	now := time.Now()
	if key.CreatedAt.IsZero() {
		key.CreatedAt = now
	}
	if key.UpdatedAt.IsZero() {
		key.UpdatedAt = now
	}

	_, err := r.collection.InsertOne(ctx, key)
	if err != nil {
		r.logger.Error("Failed to insert API key",
			zap.Error(err),
			zap.String("key_id", key.ID.Hex()),
			zap.String("key_name", key.Name),
		)
		return err
	}

	r.logger.Info("API key created successfully",
		zap.String("key_id", key.ID.Hex()),
		zap.String("key_name", key.Name),
	)

	return nil
}

// GetByID retrieves an API key by its ID
func (r *APIKeyRepository) GetByID(ctx context.Context, id string) (*models.APIKey, error) {
	r.logger.Debug("Retrieving API key by ID", zap.String("key_id", id))

	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		r.logger.Error("Invalid ObjectID format",
			zap.Error(err),
			zap.String("key_id", id),
		)
		return nil, err
	}

	var key models.APIKey
	err = r.collection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&key)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			r.logger.Warn("API key not found", zap.String("key_id", id))
			return nil, err
		}
		r.logger.Error("Failed to retrieve API key",
			zap.Error(err),
			zap.String("key_id", id),
		)
		return nil, err
	}

	r.logger.Debug("API key retrieved successfully",
		zap.String("key_id", id),
		zap.String("key_name", key.Name),
	)

	return &key, nil
}

// GetByKeyHash retrieves an API key by its hash
func (r *APIKeyRepository) GetByKeyHash(ctx context.Context, hash string) (*models.APIKey, error) {
	r.logger.Debug("Retrieving API key by hash",
		zap.String("hash_prefix", hash[:min(16, len(hash))]+"..."),
	)

	var key models.APIKey
	err := r.collection.FindOne(ctx, bson.M{"key_hash": hash}).Decode(&key)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			r.logger.Debug("API key not found by hash")
			return nil, err
		}
		r.logger.Error("Failed to retrieve API key by hash",
			zap.Error(err),
		)
		return nil, err
	}

	r.logger.Debug("API key retrieved by hash",
		zap.String("key_id", key.ID.Hex()),
		zap.String("key_name", key.Name),
	)

	return &key, nil
}

// Update updates an existing API key
func (r *APIKeyRepository) Update(ctx context.Context, key *models.APIKey) error {
	r.logger.Info("Updating API key",
		zap.String("key_id", key.ID.Hex()),
		zap.String("key_name", key.Name),
	)

	key.UpdatedAt = time.Now()

	update := bson.M{"$set": key}
	_, err := r.collection.UpdateOne(ctx, bson.M{"_id": key.ID}, update)
	if err != nil {
		r.logger.Error("Failed to update API key",
			zap.Error(err),
			zap.String("key_id", key.ID.Hex()),
		)
		return err
	}

	r.logger.Info("API key updated successfully",
		zap.String("key_id", key.ID.Hex()),
		zap.String("key_name", key.Name),
	)

	return nil
}

// UpdateStatus updates the active status of an API key
func (r *APIKeyRepository) UpdateStatus(ctx context.Context, id string, isActive bool) error {
	r.logger.Info("Updating API key status",
		zap.String("key_id", id),
		zap.Bool("is_active", isActive),
	)

	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		r.logger.Error("Invalid ObjectID format",
			zap.Error(err),
			zap.String("key_id", id),
		)
		return err
	}

	update := bson.M{
		"$set": bson.M{
			"is_active":  isActive,
			"updated_at": time.Now(),
		},
	}

	result, err := r.collection.UpdateOne(ctx, bson.M{"_id": objectID}, update)
	if err != nil {
		r.logger.Error("Failed to update API key status",
			zap.Error(err),
			zap.String("key_id", id),
		)
		return err
	}

	if result.MatchedCount == 0 {
		r.logger.Warn("API key not found for status update",
			zap.String("key_id", id),
		)
		return mongo.ErrNoDocuments
	}

	r.logger.Info("API key status updated successfully",
		zap.String("key_id", id),
		zap.Bool("is_active", isActive),
	)

	return nil
}

// UpdateUsage increments the usage count and updates last used timestamp
func (r *APIKeyRepository) UpdateUsage(ctx context.Context, id string) error {
	r.logger.Debug("Updating API key usage", zap.String("key_id", id))

	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		r.logger.Error("Invalid ObjectID format",
			zap.Error(err),
			zap.String("key_id", id),
		)
		return err
	}

	now := time.Now()
	update := bson.M{
		"$set": bson.M{
			"last_used_at": now,
			"updated_at":   now,
		},
		"$inc": bson.M{
			"usage_count": 1,
		},
	}

	result, err := r.collection.UpdateOne(ctx, bson.M{"_id": objectID}, update)
	if err != nil {
		r.logger.Error("Failed to update API key usage",
			zap.Error(err),
			zap.String("key_id", id),
		)
		return err
	}

	if result.MatchedCount == 0 {
		r.logger.Warn("API key not found for usage update",
			zap.String("key_id", id),
		)
		return mongo.ErrNoDocuments
	}

	r.logger.Debug("API key usage updated successfully",
		zap.String("key_id", id),
	)

	return nil
}

// List retrieves all API keys with optional filtering
func (r *APIKeyRepository) List(ctx context.Context) ([]models.APIKey, error) {
	r.logger.Debug("Listing API keys")

	// Default options - could be made configurable
	opts := options.Find().SetSort(bson.M{"created_at": -1})

	cursor, err := r.collection.Find(ctx, bson.M{}, opts)
	if err != nil {
		r.logger.Error("Failed to list API keys", zap.Error(err))
		return nil, err
	}
	defer cursor.Close(ctx)

	var keys []models.APIKey
	if err = cursor.All(ctx, &keys); err != nil {
		r.logger.Error("Failed to decode API keys", zap.Error(err))
		return nil, err
	}

	r.logger.Info("API keys listed successfully",
		zap.Int("count", len(keys)),
	)

	return keys, nil
}

// Delete removes an API key from the database
func (r *APIKeyRepository) Delete(ctx context.Context, id string) error {
	r.logger.Info("Deleting API key", zap.String("key_id", id))

	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		r.logger.Error("Invalid ObjectID format",
			zap.Error(err),
			zap.String("key_id", id),
		)
		return err
	}

	result, err := r.collection.DeleteOne(ctx, bson.M{"_id": objectID})
	if err != nil {
		r.logger.Error("Failed to delete API key",
			zap.Error(err),
			zap.String("key_id", id),
		)
		return err
	}

	if result.DeletedCount == 0 {
		r.logger.Warn("API key not found for deletion",
			zap.String("key_id", id),
		)
		return mongo.ErrNoDocuments
	}

	r.logger.Info("API key deleted successfully",
		zap.String("key_id", id),
	)

	return nil
}

// CreateIndexes ensures the necessary indexes are created
func (r *APIKeyRepository) CreateIndexes(ctx context.Context) error {
	r.logger.Info("Creating API key indexes")

	indexes := []mongo.IndexModel{
		{
			Keys:    bson.M{"key_hash": 1},
			Options: options.Index().SetUnique(true).SetName("key_hash_unique"),
		},
		{
			Keys:    bson.M{"name": 1},
			Options: options.Index().SetUnique(true).SetName("name_unique"),
		},
		{
			Keys:    bson.M{"is_active": 1},
			Options: options.Index().SetName("is_active"),
		},
		{
			Keys:    bson.M{"created_at": -1},
			Options: options.Index().SetName("created_at_desc"),
		},
		{
			Keys:    bson.M{"expires_at": 1},
			Options: options.Index().SetName("expires_at"),
		},
	}

	_, err := r.collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		r.logger.Error("Failed to create indexes", zap.Error(err))
		return err
	}

	r.logger.Info("API key indexes created successfully")
	return nil
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
