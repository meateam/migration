package migration

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"google.golang.org/grpc"

	spb "github.com/meateam/search-service/proto"
)

// Search is a struct to handle the migration for Search-Service.
type Search struct {
	fileDB *mongo.Database
	client spb.SearchClient
}

// FileBSON is a struct of the file document that is stored in MongoDB.
type FileBSON struct {
	ID          primitive.ObjectID  `bson:"_id,omitempty"`
	Description string              `bson:"description"`
	Size        int64               `bson:"size"`
	Parent      *primitive.ObjectID `bson:"parent,omitempty"`
	Float       bool                `bson:"float"`
	Type        string              `bson:"type,omitempty"`
	Name        string              `bson:"name,omitempty"`
	OwnerID     string              `bson:"ownerID,omitempty"`
	Bucket      string              `bson:"bucket,omitempty"`
	Key         string              `bson:"key,omitempty"`
	CreatedAt   primitive.DateTime  `bson:"createdAt,omitempty"`
	UpdatedAt   primitive.DateTime  `bson:"updatedAt,omitempty"`
}

// NewSearchMigration returns a Search migration.
func NewSearchMigration(fileConnectionString string, searchServiceURL string) *Search {
	searchConn, err := grpc.Dial(
		searchServiceURL,
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(10<<20)),
		grpc.WithInsecure())
	if err != nil {
		panic(fmt.Errorf("failed dialing search service at %s: %v", searchServiceURL, err))
	}

	searchClient := spb.NewSearchClient(searchConn)

	mongoClient, err := connectToMongoDB(fileConnectionString)
	if err != nil {
		panic(fmt.Errorf("failed connecting to %s: %v", fileConnectionString, err))
	}

	fileDB, err := getMongoDatabaseName(mongoClient, fileConnectionString)
	if err != nil {
		panic(fmt.Errorf("failed getting DB name from %s: %v", fileConnectionString, err))
	}

	return &Search{fileDB: fileDB, client: searchClient}
}

// Run runs the Search-Service migration.
func (s *Search) Run(errc chan error) {
	collection := s.fileDB.Collection(FileServiceCollectionName)
	cur, err := collection.Find(context.Background(), bson.D{})
	defer cur.Close(context.Background())
	if err != nil {
		errc <- err
		return
	}

	files := []*FileBSON{}
	for cur.Next(context.Background()) {
		file := &FileBSON{}
		err := cur.Decode(file)
		if err != nil {
			errc <- err
			return
		}

		files = append(files, file)
	}

	if err := cur.Err(); err != nil {
		errc <- err
		return
	}

	for _, file := range files {
		searchFile := &spb.File{
			Id:          file.ID.Hex(),
			Key:         file.Key,
			Bucket:      file.Bucket,
			Name:        file.Name,
			Type:        file.Type,
			Description: file.Description,
			OwnerID:     file.OwnerID,
			Size:        file.Size,
			CreatedAt:   file.CreatedAt.Time().Unix(),
			UpdatedAt:   file.UpdatedAt.Time().Unix(),
			Children:    nil,
		}

		if file.Parent != nil {
			searchFile.FileOrId = &spb.File_Parent{Parent: file.Parent.Hex()}
		}

		if _, err := s.client.CreateFile(context.Background(), searchFile); err != nil {
			errc <- err
			return
		}
	}
}
