package repository

import (
	"errors"
	"rate-limit-service/model"
	"rate-limit-service/store"

	"gorm.io/gorm"
)

func GetActiveAlgorithm() (*model.Algorithm, error) {
	var algorithm model.Algorithm
	result := store.DB.Where("is_active = ?", true).Order("priority DESC").First(&algorithm)

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}

	return &algorithm, result.Error
}
