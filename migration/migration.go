package migration

import (
	"rate-limit-service/model"
	"rate-limit-service/store"
)

func MigrateDB() {
	store.DB.AutoMigrate(
		&model.Algorithm{},
		&model.APIKey{},
	)
}
