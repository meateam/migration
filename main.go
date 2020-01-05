package main

import (
	"fmt"
	migration "github.com/meateam/migration/migrations"
	"os"
	"sync"
)

const (
	envFileMongoHost       = "FILE_MONGO_CONN_STRING"
	envFileServiceURL      = "FILE_SERVICE_URL"
	envPermissionMongoHost = "PERMISSION_MONGO_CONN_STRING"
	envSearchServiceURL    = "SEARCH_SERVICE_URL"
)

func main() {
	// Init File-Service MongoDB client connection.
	fileMongoConnectionString := os.Getenv(envFileMongoHost)
	if fileMongoConnectionString == "" {
		panic("os.Getenv(envFileMongoHost) = \"\"")
	}

	fileMigration := migration.NewFileMigration(fileMongoConnectionString)

	// Init Permission-Service MongoDB client connection.
	permissionMongoConnectionString := os.Getenv(envPermissionMongoHost)
	if permissionMongoConnectionString == "" {
		panic("os.Getenv(envPermissionMongoHost) = \"\"")
	}

	// Init File-Service client connection.
	fileServiceURL := os.Getenv(envFileServiceURL)
	if fileServiceURL == "" {
		panic("os.Getenv(fileServiceURL) = \"\"")
	}

	permissionMigration := migration.NewPermissionMigration(permissionMongoConnectionString, fileServiceURL)

	ssURL := os.Getenv(envSearchServiceURL)
	if ssURL == "" {
		panic("os.Getenv(envSearchServiceURL) = \"\"")
	}

	searchMigration := migration.NewSearchMigration(fileMongoConnectionString, ssURL)

	wg := sync.WaitGroup{}
	wg.Add(3)

	errc := make(chan error, 5)

	// Run File Migration.
	go func() {
		defer wg.Done()
		fileMigration.Run(errc)
	}()

	// Run Permission Migration.
	go func() {
		defer wg.Done()
		permissionMigration.Run(errc)
	}()

	// Run Search Migration.
	go func() {
		wg.Done()
		searchMigration.Run(errc)
	}()

	wg.Wait()

	for i := 0; i < 5; i++ {
		select {
		case err := <-errc:
			fmt.Println(err)
		default:
		}
	}

	fmt.Println("Migration done")
}
