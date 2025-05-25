package app

import (
	"bytes"
	"fmt"
	tele "gopkg.in/telebot.v4"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"image"
	"io"
	"log"
	"net/http"
	"shakalizator/internal/app/config"
	"shakalizator/internal/app/models"
	"shakalizator/internal/app/repository"
	"shakalizator/internal/app/shakalizator"
	"shakalizator/pkg/logger"
	"strconv"
	"strings"
	"time"
)

var fileIDs = make(map[int64]string)

type App struct {
	log       *logger.Logger
	cfg       *config.Config
	db        *gorm.DB
	statsRepo *repository.StatsRepository
	bot       *tele.Bot
}

func New() error {
	a := &App{
		log: logger.New(),
	}

	var err error
	a.cfg, err = config.NewConfig()
	if err != nil {
		a.log.Error("Error loading config from env", err)
		return err
	}
	a.log.SetLogLevel(a.cfg.LoggerLevel)

	a.db, err = gorm.Open(sqlite.Open("shakalizator.db"), &gorm.Config{})
	if err != nil {
		return err
	}
	err = a.db.AutoMigrate(models.Chat{}, models.Event{})
	if err != nil {
		return err
	}

	a.statsRepo = repository.NewStats(a.db)
	go a.statsRepo.EventLoop()

	return RunBot(a)
}

func RunBot(a *App) error {
	pref := tele.Settings{
		Token:  a.cfg.TelegramAPI,
		Poller: &tele.LongPoller{Timeout: 1 * time.Second},
	}

	b, err := tele.NewBot(pref)
	if err != nil {
		return err
	}

	b.Handle("/start", func(c tele.Context) error {
		return c.Send(fmt.Sprintf("Привет, %s!\n\nЯ - Shakalizator. Просто отправь мне картинку или видео, чтобы зашакалить её", c.Sender().FirstName))
	})

	b.Handle(tele.OnPhoto, func(c tele.Context) error {
		fileIDs[c.Sender().ID] = c.Message().Photo.FileID
		menu := &tele.ReplyMarkup{}

		var rows []tele.Row
		for id := 1; id <= 10; id++ {
			if (id-1)%5 == 0 {
				rows = append(rows, tele.Row{})
			}
			btn := menu.Data(strconv.Itoa(id), fmt.Sprintf("level_%d", id))
			rows[len(rows)-1] = append(rows[len(rows)-1], btn)
		}

		menu.Inline(rows...)
		return c.Reply("Выбери уровень шакализации:", menu)
	})

	b.Handle(tele.OnCallback, func(c tele.Context) error {
		go func() {
			if err := a.statsRepo.RecordEvent(c.Chat().ID); err != nil {
				a.log.Error("Failed to record event", err)
			}
		}()

		fileID := fileIDs[c.Sender().ID]
		if fileID == "" {
			return nil
		}

		level, err := strconv.Atoi(strings.TrimPrefix(strings.TrimSpace(c.Callback().Data), "level_"))
		if err != nil || level < 1 || level > 10 {
			log.Println("Level out of range or err != nil", err)
			return c.Send("Ошибка! Повторите попытку позже")
		}

		file, err := b.FileByID(fileID)
		if err != nil {
			log.Println(err)
			return c.Send("Ошибка! Повторите попытку позже")
		}

		fileIDs[c.Sender().ID] = ""
		downloadURL := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", b.Token, file.FilePath)

		resp, err := http.Get(downloadURL)
		if err != nil {
			log.Println(err)
			return c.Send("Ошибка при загрузке фото: " + err.Error())
		}
		defer resp.Body.Close()

		imgData, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Println(err)
			return c.Send("Ошибка при чтении фото: " + err.Error())
		}

		img, _, err := image.Decode(bytes.NewReader(imgData))
		if err != nil {
			log.Println(err)
			return c.Send("Ошибка при декодировании фото: " + err.Error())
		}
		bufImg := shakalizator.ShakalizeImage(img, level)

		return c.Send(&tele.Photo{
			File:    tele.FromReader(bufImg),
			Caption: "@ultrashakal_bot",
		})
	})

	b.Handle("/stats", func(c tele.Context) error {
		if c.Message().Sender.ID != 1230045591 {
			return nil
		}

		a.statsRepo.Flush()

		statsHour, err := a.statsRepo.GetStats("hour")
		if err != nil {
			return err
		}

		statsDay, err := a.statsRepo.GetStats("day")
		if err != nil {
			return err
		}

		statsWeek, err := a.statsRepo.GetStats("week")
		if err != nil {
			return err
		}

		statsMonth, err := a.statsRepo.GetStats("month")
		if err != nil {
			return err
		}

		count, err := a.statsRepo.GetActiveChatsCount()
		if err != nil {
			return err
		}

		msg := fmt.Sprintf("📊 Статистика по количеству использований:\n"+
			"👉 за последний час: %d\n👉 за последний день: %d\n👉 за последнюю неделю: %d\n👉 за последний месяц: %d\n\n"+
			"🚀 Количество пользователей (за всё время): %d", statsHour, statsDay, statsWeek, statsMonth, count)
		return c.Send(msg, &tele.SendOptions{ReplyTo: c.Message()})
	})

	b.Start()
	return nil
}
