package models

import "time"

type Chat struct {
	ID        int64     `gorm:"primaryKey"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

type Event struct {
	ID        uint      `gorm:"primaryKey;autoIncrement"`
	ChatID    int64     `gorm:"index"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}
