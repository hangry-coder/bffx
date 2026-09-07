package cli

import (
	"fmt"
	"log"
	"github.com/hangry-coder/bffx/pkg/app"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/storage"
	"github.com/hangry-coder/bffx/pkg/storage/schema"
)

func HandleSeed(args []string) {
	root := "."
	upsert := false
	for i := 0; i < len(args); i++ {
		if args[i] == "--root" && i+1 < len(args) {
			root = args[i+1]
			i++
		}
		if args[i] == "--upsert" {
			upsert = true
		}
	}

	app.LoadEnv(root)

	reg, err := manifest.LoadAll(root)
	if err != nil {
		log.Fatalf("failed to load manifests: %v", err)
	}

	projectSpec := reg.ProjectSpec()
	store, err := storage.NewStore(root, projectSpec, reg)
	if err != nil {
		log.Fatalf("failed to open store: %v", err)
	}

	migrationsDir := migrationsDirForRoot(root, projectSpec)
	if schema.MigrationsAdopted(migrationsDir) {
		if sqlDB, driverName := resolveSQLDBFromStore(store); sqlDB != nil {
			if err := applyMigrations(sqlDB, driverName, migrationsDir); err != nil {
				log.Fatalf("migrate apply before seed: %v", err)
			}
		}
	}

	fmt.Printf("Seeding data for project: %s...\n", reg.Project.Metadata.Name)
	if err := storage.SeedRegistry(reg, store, storage.SeedOptions{Upsert: upsert}); err != nil {
		log.Fatalf("seeding failed: %v", err)
	}
	fmt.Println("Seeding completed successfully.")
}
