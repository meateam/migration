package migration

import (
	"context"
	"fmt"
	"sync"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// FileServiceCollectionName is the name of files collection.
const FileServiceCollectionName = "files"

// File is a struct to handle the migration for File-Service.
type File struct {
	db *mongo.Database
}

// NewFileMigration returns a File migration.
func NewFileMigration(fileConnectionString string) *File {
	mongoClient, err := connectToMongoDB(fileConnectionString)
	if err != nil {
		panic(fmt.Errorf("failed connecting to %s: %v", fileConnectionString, err))
	}

	db, err := getMongoDatabaseName(mongoClient, fileConnectionString)
	if err != nil {
		panic(fmt.Errorf("failed getting DB name from %s: %v", fileConnectionString, err))
	}

	return &File{db}
}

// Run runs the File-Service migration.
func (f *File) Run(errc chan error) {
	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		defer wg.Done()
		if err := f.RunSetFloat(); err != nil {
			errc <- err
		}
	}()

	go func() {
		defer wg.Done()
		if err := f.RunUpdateNameParentOwnerIndex(); err != nil {
			errc <- err
		}
	}()

	wg.Wait()
}

// RunUpdateNameParentOwnerIndex drops the old name_1_parent_1_ownerID_1 unique index,
// and creates the new not-unique index.
func (f *File) RunUpdateNameParentOwnerIndex() error {
	indexModel := mongo.IndexModel{
		Keys: bson.D{
			bson.E{
				Key:   "name",
				Value: 1,
			},
			bson.E{
				Key:   "parent",
				Value: 1,
			},
			bson.E{
				Key:   "ownerID",
				Value: 1,
			},
		},
		Options: options.Index().SetBackground(true),
	}
	collection := f.db.Collection(FileServiceCollectionName)
	if _, err := collection.Indexes().DropOne(context.Background(), "name_1_parent_1_ownerID_1"); err != nil {
		return fmt.Errorf("failed dropping old index name_1_parent_1_ownerID_1 for File-Service: %v", err)
	}

	name, err := collection.Indexes().CreateOne(context.Background(), indexModel)
	if err != nil {
		return fmt.Errorf("failed creating index name_1_parent_1_ownerID_1 for File-Service: %v", err)
	}

	if name != "name_1_parent_1_ownerID_1" {
		return fmt.Errorf("unexpected created index name: expected: name_1_parent_1_ownerID_1 but got: %s", name)
	}

	return nil
}

// RunSetFloat runs the setting of the float field to false for all files documents where it doesn't exist.
func (f *File) RunSetFloat() error {
	collection := f.db.Collection(FileServiceCollectionName)
	filter := bson.D{
		bson.E{
			Key: "float",
			Value: bson.D{
				bson.E{
					Key:   "$exists",
					Value: false,
				},
			},
		},
	}

	update := bson.D{
		bson.E{
			Key: "$set",
			Value: bson.D{
				bson.E{
					Key:   "float",
					Value: false,
				},
			},
		},
	}

	_, err := collection.UpdateMany(context.Background(), filter, update)
	if err != nil {
		return fmt.Errorf("failed setting float: false for File-Service: %v", err)
	}

	return nil
}
