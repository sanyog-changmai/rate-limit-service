package model

import (
	"time"

	"github.com/google/uuid"
)

type APIKey struct {
	ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	APIKey    string    `gorm:"type:varchar(255);uniqueIndex;not null"`
	Name      string    `gorm:"type:varchar(255);not null"`
	Rate      float64   `gorm:"not null"`
	Burst     int       `gorm:"not null"`
	IsActive  bool      `gorm:"not null;default:true"`
	CreatedAt time.Time
	UpdatedAt time.Time
}
