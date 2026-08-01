package history

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"streamly/internal/resolve"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	databaseName = "streamly-redux"
	collectionName = "history"

	// Cap how much we keep per Discord server.
	MaxEntries = 50

	// Don't offer resume for trivial progress or near-complete watches.
	minResumeMs = 30_000
	endGuardMs = 120_000
)

// Entry is one recently-played title or channel for a guild.
type Entry struct {

	GuildID string `json:"guildId" bson:"guildId"`
	Key string `json:"key" bson:"key"`

	Kind string `json:"kind" bson:"kind"`
	ID string `json:"id" bson:"id"`

	Title string `json:"title" bson:"title"`
	Poster string `json:"poster,omitempty" bson:"poster,omitempty"`
	Caption string `json:"caption,omitempty" bson:"caption,omitempty"`
	Description string `json:"description,omitempty" bson:"description,omitempty"`

	BoxType int `json:"boxType,omitempty" bson:"boxType,omitempty"`

	Season int `json:"season,omitempty" bson:"season,omitempty"`
	Episode int `json:"episode,omitempty" bson:"episode,omitempty"`
	EpisodeTitle string `json:"episodeTitle,omitempty" bson:"episodeTitle,omitempty"`

	// VOD only — last known shared position for this server.
	PositionMs int64 `json:"positionMs,omitempty" bson:"positionMs,omitempty"`
	DurationMs int64 `json:"durationMs,omitempty" bson:"durationMs,omitempty"`

	PlayedAt time.Time `json:"playedAt" bson:"playedAt"`

}

type Store struct {

	col *mongo.Collection

}

func Open(ctx context.Context, uri string) (*Store, error) {

	if uri == "" {

		return nil, fmt.Errorf("MONGO_URI is empty")

	}

	client, err := mongo.Connect(options.Client().ApplyURI(uri))

	if err != nil {

		return nil, err

	}

	if err := client.Ping(ctx, nil); err != nil {

		_ = client.Disconnect(ctx)
		return nil, err

	}

	col := client.Database(databaseName).Collection(collectionName)

	store := &Store{col: col}

	if err := store.ensureIndexes(ctx); err != nil {

		slog.Warn("history indexes failed", "err", err)

	}

	return store, nil

}

func (s *Store) ensureIndexes(ctx context.Context) error {

	models := []mongo.IndexModel{

		{

			Keys: bson.D{{Key: "guildId", Value: 1}, {Key: "key", Value: 1}},
			Options: options.Index().SetUnique(true),

		},

		{

			Keys: bson.D{{Key: "guildId", Value: 1}, {Key: "playedAt", Value: -1}},

		},

	}

	_, err := s.col.Indexes().CreateMany(ctx, models)

	return err

}

// Record bumps a title/channel to the top of the server's history (no position change).
func (s *Store) Record(ctx context.Context, guildID string, item resolve.Item) error {

	if s == nil || guildID == "" || item.ID == "" {

		return nil

	}

	key := historyKey(item)
	now := time.Now().UTC()

	set := bson.M{

		"guildId": guildID,
		"key": key,

		"kind": string(item.Kind),
		"id": item.ID,

		"title": item.Title,
		"caption": item.Caption,
		"boxType": item.BoxType,

		"season": item.Season,
		"episode": item.Episode,
		"episodeTitle": item.EpisodeTitle,

		"playedAt": now,

	}

	// Prefer a stable title/channel poster; never blank an existing one with an empty value.
	if item.Poster != "" {

		set["poster"] = item.Poster

	}

	if item.Description != "" {

		set["description"] = item.Description

	}

	update := bson.M{

		"$set": set,

		"$setOnInsert": bson.M{

			"positionMs": int64(0),
			"durationMs": int64(0),

		},

	}

	opts := options.UpdateOne().SetUpsert(true)

	if _, err := s.col.UpdateOne(ctx, bson.M{"guildId": guildID, "key": key}, update, opts); err != nil {

		return err

	}

	return s.prune(ctx, guildID)

}

// SaveProgress updates VOD watch position for an existing (or new) history row.
func (s *Store) SaveProgress(ctx context.Context, guildID string, item resolve.Item, positionMs, durationMs int64) error {

	if s == nil || guildID == "" || item.ID == "" || item.Kind != resolve.KindVOD {

		return nil

	}

	if positionMs < 0 {

		positionMs = 0

	}

	if durationMs < 0 {

		durationMs = 0

	}

	key := historyKey(item)
	now := time.Now().UTC()

	set := bson.M{

		"guildId": guildID,
		"key": key,

		"kind": string(item.Kind),
		"id": item.ID,

		"title": item.Title,
		"caption": item.Caption,
		"boxType": item.BoxType,

		"season": item.Season,
		"episode": item.Episode,
		"episodeTitle": item.EpisodeTitle,

		"positionMs": positionMs,
		"durationMs": durationMs,

		"playedAt": now,

	}

	if item.Poster != "" {

		set["poster"] = item.Poster

	}

	if item.Description != "" {

		set["description"] = item.Description

	}

	update := bson.M{

		"$set": set,

	}

	opts := options.UpdateOne().SetUpsert(true)

	if _, err := s.col.UpdateOne(ctx, bson.M{"guildId": guildID, "key": key}, update, opts); err != nil {

		return err

	}

	return s.prune(ctx, guildID)

}

// ClearProgress zeroes the saved position (e.g. user chose "Start over").
func (s *Store) ClearProgress(ctx context.Context, guildID string, item resolve.Item) error {

	if s == nil || guildID == "" || item.ID == "" {

		return nil

	}

	_, err := s.col.UpdateOne(ctx, bson.M{"guildId": guildID, "key": historyKey(item)}, bson.M{

		"$set": bson.M{"positionMs": int64(0)},

	})

	return err

}

func (s *Store) List(ctx context.Context, guildID string, limit int) ([]Entry, error) {

	if s == nil || guildID == "" {

		return nil, nil

	}

	if limit <= 0 || limit > MaxEntries {

		limit = MaxEntries

	}

	opts := options.Find().SetSort(bson.D{{Key: "playedAt", Value: -1}}).SetLimit(int64(limit))

	cursor, err := s.col.Find(ctx, bson.M{"guildId": guildID}, opts)

	if err != nil {

		return nil, err

	}

	defer cursor.Close(ctx)

	var entries []Entry

	if err := cursor.All(ctx, &entries); err != nil {

		return nil, err

	}

	if entries == nil {

		entries = []Entry{}

	}

	return entries, nil

}

// ResumePosition returns a savable VOD position if it is worth offering.
func (s *Store) ResumePosition(ctx context.Context, guildID string, item resolve.Item) (positionMs int64, durationMs int64, ok bool) {

	if s == nil || guildID == "" || item.Kind != resolve.KindVOD {

		return 0, 0, false

	}

	var entry Entry

	err := s.col.FindOne(ctx, bson.M{"guildId": guildID, "key": historyKey(item)}).Decode(&entry)

	if err != nil {

		return 0, 0, false

	}

	if entry.PositionMs < minResumeMs {

		return 0, 0, false

	}

	if entry.DurationMs > 0 && entry.PositionMs+endGuardMs >= entry.DurationMs {

		return 0, 0, false

	}

	return entry.PositionMs, entry.DurationMs, true

}

func (s *Store) prune(ctx context.Context, guildID string) error {

	opts := options.Find().
		SetSort(bson.D{{Key: "playedAt", Value: -1}}).
		SetSkip(int64(MaxEntries)).
		SetProjection(bson.M{"_id": 1})

	cursor, err := s.col.Find(ctx, bson.M{"guildId": guildID}, opts)

	if err != nil {

		return err

	}

	defer cursor.Close(ctx)

	var stale []struct {
		ID bson.ObjectID `bson:"_id"`
	}

	if err := cursor.All(ctx, &stale); err != nil || len(stale) == 0 {

		return err

	}

	ids := make([]bson.ObjectID, len(stale))

	for i, row := range stale {

		ids[i] = row.ID

	}

	_, err = s.col.DeleteMany(ctx, bson.M{"_id": bson.M{"$in": ids}})

	return err

}

func historyKey(item resolve.Item) string {

	if item.Kind == resolve.KindChannel {

		return "channel:" + item.ID

	}

	// Episode-scoped so resume position is correct per installment.
	return fmt.Sprintf("vod:%s:%d:%d", item.ID, item.Season, item.Episode)

}
