package repository

import (
	"database/sql"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"log/slog"
	"shakalizator/internal/app/models"
	"shakalizator/pkg/logger"
	"strings"
	"sync"
	"time"
)

type StatsRepository struct {
	log         *logger.Logger
	db          *gorm.DB
	eventsChan  chan int64
	flushTicker *time.Ticker
	done        chan struct{}
	buffer      []int64
	bufferMutex sync.Mutex
}

func NewStats(db *gorm.DB) *StatsRepository {
	return &StatsRepository{
		db:          db,
		eventsChan:  make(chan int64, 1000),
		flushTicker: time.NewTicker(5 * time.Minute),
		done:        make(chan struct{}),
		buffer:      make([]int64, 0, 1000),
	}
}

func (sr *StatsRepository) EventLoop() {
	for {
		select {
		case chatID := <-sr.eventsChan:
			sr.bufferMutex.Lock()
			sr.buffer = append(sr.buffer, chatID)
			if len(sr.buffer) >= 1000 {
				sr.flushBuffer()
			}
			sr.bufferMutex.Unlock()

		case <-sr.flushTicker.C:
			sr.bufferMutex.Lock()
			if len(sr.buffer) > 0 {
				sr.flushBuffer()
			}
			sr.bufferMutex.Unlock()

		case <-sr.done:
			sr.flushTicker.Stop()
			sr.bufferMutex.Lock()
			sr.flushBuffer()
			sr.bufferMutex.Unlock()
			return
		}
	}
}

func (sr *StatsRepository) Flush() {
	sr.bufferMutex.Lock()
	defer sr.bufferMutex.Unlock()

	if len(sr.buffer) > 0 {
		sr.flushBuffer()
	}
}

func (sr *StatsRepository) flushBuffer() {
	defer func() {
		if r := recover(); r != nil {
			sr.log.Error("panic in flushBuffer", nil, slog.Any("recover", r))
		}
	}()

	if len(sr.buffer) == 0 {
		return
	}

	chatCounts := make(map[int64]int)
	for _, chatID := range sr.buffer {
		chatCounts[chatID]++
	}

	tx := sr.db.Begin()
	if tx.Error != nil {
		sr.log.Error("Failed to begin transaction", tx.Error)
		return
	}

	for chatID := range chatCounts {
		chat := models.Chat{ID: chatID}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoUpdates: clause.Assignments(map[string]interface{}{"updated_at": gorm.Expr("CURRENT_TIMESTAMP")}),
		}).Create(&chat).Error; err != nil {
			sr.log.Error("Failed to upsert chat", err)
			tx.Rollback()
			return
		}
	}

	events := make([]models.Event, 0, len(sr.buffer))
	for _, chatID := range sr.buffer {
		events = append(events, models.Event{ChatID: chatID})
	}
	batchSize := 999
	for i := 0; i < len(events); i += batchSize {
		end := i + batchSize
		if end > len(events) {
			end = len(events)
		}
		if err := tx.Create(events[i:end]).Error; err != nil {
			sr.log.Error("Failed to batch insert events", err)
			tx.Rollback()
			return
		}
	}

	if err := tx.Commit().Error; err != nil {
		sr.log.Error("Failed to commit transaction", err)
		return
	}

	sr.buffer = sr.buffer[:0]
}

func (sr *StatsRepository) batchInsert(tx *sql.Tx, valueStrings []string, valueArgs []interface{}) {
	stmt := fmt.Sprintf(
		"INSERT INTO events (chat_id, created_at) VALUES %s",
		strings.Join(valueStrings, ","),
	)

	_, err := tx.Exec(stmt, valueArgs...)
	if err != nil {
		sr.log.Error("Failed to batch insert events", err)
	}
}

func (sr *StatsRepository) RecordEvent(chatID int64) error {
	select {
	case sr.eventsChan <- chatID:
	default:
		sr.log.Warn("events channel is full, dropping event")
	}
	return nil
}

func (sr *StatsRepository) GetStats(period string) (int, error) {
	var timeInterval time.Duration
	switch period {
	case "hour":
		timeInterval = -1 * time.Hour
	case "day":
		timeInterval = -24 * time.Hour
	case "week":
		timeInterval = -7 * 24 * time.Hour
	case "month":
		timeInterval = -30 * 24 * time.Hour
	default:
		return 0, fmt.Errorf("invalid period: %s", period)
	}

	startTime := time.Now().Add(timeInterval)
	var count int64
	if err := sr.db.Model(&models.Event{}).
		Where("created_at >= ?", startTime).
		Count(&count).Error; err != nil {
		return 0, err
	}

	return int(count), nil
}

func (sr *StatsRepository) GetActiveChatsCount() (int, error) {
	var count int64
	if err := sr.db.Model(&models.Chat{}).
		Distinct("id").
		Count(&count).Error; err != nil {
		return 0, err
	}
	return int(count), nil
}
