package model

import "time"

type Algorithm struct {
	ID        uint      `gorm:"primaryKey;autoIncrement"`
	Name      string    `gorm:"type:varchar(50);uniqueIndex;not null"`
	IsActive  bool      `gorm:"not null;default:false"`
	Priority  int       `gorm:"not null;default:0"`
	CreatedAt time.Time
	UpdatedAt time.Time
}
