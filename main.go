package main

import (
	"bytes"
	"fmt"
	"github.com/caarlos0/env"
	"github.com/joho/godotenv"
	"image"
	"image/jpeg"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	tele "gopkg.in/telebot.v4"
)

var fileIDs = map[int64]string{}

func main() {
	cfg, err := newConfig()
	if err != nil {
		panic(err)
	}

	pref := tele.Settings{
		Token:  cfg.TelegramAPI,
		Poller: &tele.LongPoller{Timeout: 1 * time.Second},
	}

	b, err := tele.NewBot(pref)
	if err != nil {
		panic(err)
	}

	b.Handle("/start", func(c tele.Context) error {
		return c.Send(fmt.Sprintf("Привет, %s!\n\nЯ - Shakalizator. Просто отправь мне картинку, чтобы зашакалить её", c.Sender().FirstName))
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
		bufImg := shakalizeImage(img, level)

		return c.Send(&tele.Photo{
			File:    tele.FromReader(bufImg),
			Caption: "@ultrashakal_bot",
		})
	})

	b.Start()
}

func shakalizeImage(src image.Image, level int) *bytes.Buffer {
	level *= 3
	scaleFactor := 1.0 / (4.0 + float64(level)/3.0)
	jpegQuality := 31 - level

	smallWidth := int(float64(src.Bounds().Dx()) * scaleFactor)
	smallHeight := int(float64(src.Bounds().Dy()) * scaleFactor)
	small := imaging.Resize(src, smallWidth, smallHeight, imaging.Lanczos)
	large := imaging.Resize(small, src.Bounds().Dx(), src.Bounds().Dy(), imaging.NearestNeighbor)

	var buf bytes.Buffer
	err := jpeg.Encode(&buf, large, &jpeg.Options{Quality: jpegQuality})
	if err != nil {
		log.Println(err)
		return nil
	}

	return &buf
}

type Config struct {
	TelegramAPI string `env:"TELEGRAM_API,required"`
}

func newConfig(files ...string) (*Config, error) {
	err := godotenv.Load(files...)
	if err != nil {
		log.Fatal("Файл .env не найден", err.Error())
	}

	cfg := Config{}
	err = env.Parse(&cfg)
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}
