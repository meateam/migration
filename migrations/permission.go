package migration

import (
	"context"
	"fmt"
	"sync"

	fpb "github.com/meateam/file-service/proto/file"
	ppb "github.com/meateam/permission-service/proto"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"google.golang.org/grpc"
)

const permissionServiceCollectionName = "permissions"

// Permission is a struct to handle the migration for Permission-Service.
type Permission struct {
	db                *mongo.Database
	fileServiceClient fpb.FileServiceClient
}

// BSON is the structure that represents a permission as it's stored.
type BSON struct {
	ID      primitive.ObjectID `bson:"_id,omitempty"`
	FileID  string             `bson:"fileID,omitempty"`
	UserID  string             `bson:"userID,omitempty"`
	Role    ppb.Role           `bson:"role"`
	Creator string             `bson:"creator"`
}

// NewPermissionMigration returns a Permission migration.
func NewPermissionMigration(permissionConnectionString string, fileServiceURL string) *Permission {
	mongoClient, err := connectToMongoDB(permissionConnectionString)
	if err != nil {
		panic(fmt.Errorf("failed connecting to %s: %v", permissionConnectionString, err))
	}

	db, err := getMongoDatabaseName(mongoClient, permissionConnectionString)
	if err != nil {
		panic(fmt.Errorf("failed getting db name from %s: %v", permissionConnectionString, err))
	}

	fileConn, err := grpc.Dial(
		fileServiceURL,
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(10<<20)),
		grpc.WithInsecure())
	if err != nil {
		panic(fmt.Errorf("failed dialing file service at %s: %v", fileServiceURL, err))
	}

	fileServiceClient := fpb.NewFileServiceClient(fileConn)

	return &Permission{db, fileServiceClient}
}

// Run runs the Permission-Service migration.
func (p *Permission) Run(errc chan error) {
	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		defer wg.Done()
		if err := p.RunReadRoleUpdate(); err != nil {
			errc <- err
		}
	}()

	// go func() {
	// 	defer wg.Done()
	// 	if err := p.RunWriteRoleUpdate(); err != nil {
	// 		errc <- err
	// 	}
	// }()

	go func() {
		defer wg.Done()
		if err := p.RunSetCreator(); err != nil {
			errc <- err
		}
	}()

	wg.Wait()
}

// RunReadRoleUpdate runs the update read role migration.
func (p *Permission) RunReadRoleUpdate() error {
	collection := p.db.Collection(permissionServiceCollectionName)
	filterReadRole := bson.D{
		bson.E{
			Key: "role",
			Value: bson.D{
				bson.E{
					Key:   "$eq",
					Value: 3,
				},
			},
		},
	}

	updateReadRole := bson.D{
		bson.E{
			Key: "$set",
			Value: bson.D{
				bson.E{
					Key:   "role",
					Value: 2,
				},
			},
		},
	}

	_, err := collection.UpdateMany(context.Background(), filterReadRole, updateReadRole)
	if err != nil {
		return fmt.Errorf("failed updating read role from 3 to 2 in Permission-Service: %v", err)
	}

	return nil
}

// RunWriteRoleUpdate runs the update write role migration.
func (p *Permission) RunWriteRoleUpdate() error {
	collection := p.db.Collection(permissionServiceCollectionName)
	filterWriteRole := bson.D{
		bson.E{
			Key: "role",
			Value: bson.D{
				bson.E{
					Key:   "$eq",
					Value: 2,
				},
			},
		},
	}

	updateWriteRole := bson.D{
		bson.E{
			Key: "$set",
			Value: bson.D{
				bson.E{
					Key:   "role",
					Value: 1,
				},
			},
		},
	}

	_, err := collection.UpdateMany(context.Background(), filterWriteRole, updateWriteRole)
	if err != nil {
		return fmt.Errorf("failed updating write role from 2 to 1 in Permission-Service: %v", err)
	}

	return nil
}

// RunSetCreator runs the setting of the creator field to the owner of the file.
func (p *Permission) RunSetCreator() error {
	collection := p.db.Collection(permissionServiceCollectionName)

	cur, err := collection.Find(context.Background(), bson.D{})
	defer cur.Close(context.Background())
	if err != nil {
		return err
	}

	permissions := []*BSON{}
	for cur.Next(context.Background()) {
		permission := &BSON{}
		err := cur.Decode(permission)
		if err != nil {
			return err
		}

		permissions = append(permissions, permission)
	}

	if err := cur.Err(); err != nil {
		return fmt.Errorf("SetCreator cursor error: %v", err)
	}

	for _, permission := range permissions {
		file, err := p.fileServiceClient.GetFileByID(context.Background(), &fpb.GetByFileByIDRequest{Id: permission.FileID})
		if err != nil {
			return fmt.Errorf("failed getting file %s from File-Service: %v", permission.FileID, err)
		}

		filterByID := bson.D{
			bson.E{
				Key:   "_id",
				Value: permission.ID,
			},
			bson.E{
				Key: "creator",
				Value: bson.D{
					bson.E{
						Key:   "$exists",
						Value: false,
					},
				},
			},
		}

		updateCreator := bson.D{
			bson.E{
				Key: "$set",
				Value: bson.D{
					bson.E{
						Key:   "creator",
						Value: file.GetOwnerID(),
					},
				},
			},
		}

		res := collection.FindOneAndUpdate(context.Background(), filterByID, updateCreator)
		if res.Err() != nil && res.Err() != mongo.ErrNoDocuments {
			return fmt.Errorf(
				"failed setting creator for permissions %s with ownerID %s: %v",
				permission.ID, file.GetOwnerID(), res.Err())
		}
	}

	return nil
}
