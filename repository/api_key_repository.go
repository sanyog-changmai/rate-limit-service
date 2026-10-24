package repository

import (
	"errors"
	"rate-limit-service/model"
	"rate-limit-service/store"

	"gorm.io/gorm"
)

func GetPolicyByKey(apiKey string) (*model.APIKey, error) {
	var policy model.APIKey
	result := store.DB.Where("api_key = ? AND is_active = ?", apiKey, true).First(&policy)

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}

	return &policy, result.Error
}

func GetDefaultPolicy() (*model.APIKey, error) {
	var policy model.APIKey
	result := store.DB.Where("name = ? AND is_active = ?", "default", true).First(&policy)

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}

	return &policy, result.Error
}
