package migration

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.mongodb.org/mongo-driver/x/mongo/driver/connstring"
)

func connectToMongoDB(connectionString string) (*mongo.Client, error) {
	// Create mongodb client.
	mongoOptions := options.Client().ApplyURI(connectionString)
	mongoClient, err := mongo.NewClient(mongoOptions)
	if err != nil {
		return nil, fmt.Errorf("failed creating mongodb client with connection string %s: %v", connectionString, err)
	}

	// Connect client to mongodb.
	connectionTimeoutCtx, cancelConn := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancelConn()
	err = mongoClient.Connect(connectionTimeoutCtx)
	if err != nil {
		return nil, fmt.Errorf("failed connecting to mongodb with connection string %s: %v", connectionString, err)
	}

	// Check the connection.
	pingTimeoutCtx, cancelPing := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancelPing()
	err = mongoClient.Ping(pingTimeoutCtx, readpref.Primary())
	if err != nil {
		return nil, fmt.Errorf("failed pinging to mongodb with connection string %s: %v", connectionString, err)
	}

	return mongoClient, nil
}

func getMongoDatabaseName(mongoClient *mongo.Client, connectionString string) (*mongo.Database, error) {
	connString, err := connstring.Parse(connectionString)
	if err != nil {
		return nil, fmt.Errorf("failed parsing connection string %s: %v", connectionString, err)
	}

	return mongoClient.Database(connString.Database), nil
}
