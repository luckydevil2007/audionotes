package controllers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/luckydevil2007/audionotes/usecases"
)

type State int

const (
	Nothing State = iota
	ToSendVoice
	ToSendLocation
)

type InternalState struct {
	State    State
	Lat, Lon float64
}

type TelegramBot struct {
	bot      *tgbotapi.BotAPI // Экземпляр бота API Telegram
	chatID   int64            // ID чата, куда будут отправляться сообщения
	vicinity int64
	//checkAuth *usecases.AuthUseCase
	note  *usecases.NoteUseCase
	state InternalState
	//repo      *repositories.Repository
	//producer  *producers.EventProducer
}

func NewTelegramBot(token string, note *usecases.NoteUseCase) (*TelegramBot, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	// Возвращаем инициализированный адаптер с ботом и ID чата
	return &TelegramBot{bot: bot, note: note}, nil
}

func (t *TelegramBot) Test(ctx context.Context) error {
	note, err := t.note.OpenNearest(ctx, 60.0, 30.0, 1000.0)
	if err != nil {
		return err
	}
	if len(note.Data) == 0 {
		return fmt.Errorf("data==0")
	}
	return nil
}

func (t *TelegramBot) Run(ctx context.Context) error {
	u := tgbotapi.NewUpdate(0)
	updates := t.bot.GetUpdatesChan(u)
	errChan := make(chan error)
	//go func() {
	for update := range updates {
		if update.CallbackQuery != nil { // Handle button presses
			t.handleCallback(update.CallbackQuery)
		}
		if update.Message == nil {
			continue
		}

		// Handle /start command
		if update.Message.IsCommand() && update.Message.Command() == "start" {
			//	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Welcome! Use /newtour to create a tour.")
			t.sendMainMenu(update.Message.Chat.ID)
		}

		// Handle location sharing
		if update.Message.Location != nil {
			lat := float64(update.Message.Location.Latitude)
			lon := float64(update.Message.Location.Longitude)
			if t.state.State == ToSendLocation {
				t.state.Lat = lat
				t.state.Lon = lon
				t.state.State = ToSendVoice
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Now record your voice message or send an audio OGG file")
				t.bot.Send(msg)
				continue
			}
			// Check if near a tour point (pseudo-c
			note, err := t.note.OpenNearest(ctx, lat, lon, 1.0)

			if err == nil {
				file := tgbotapi.FileBytes{
					Name:  note.Title,
					Bytes: note.Data,
				}

				audioConfig := tgbotapi.NewVoice(update.Message.Chat.ID, file)
				//audio := tgbotapi.NewAudioShare(update.Message.Chat.ID, audioURL)
				t.bot.Send(audioConfig)
			}
		}

		if update.Message.Voice != nil || update.Message.Audio != nil {
			var fileID string
			if update.Message.Voice != nil {
				fileID = update.Message.Voice.FileID
			} else {
				fileID = update.Message.Audio.FileID
			}
			fileConfig := tgbotapi.FileConfig{FileID: fileID}
			file, err := t.bot.GetFile(fileConfig)
			if err != nil {
				continue
			}
			nameTmp := t.bot.Token + "/" + file.FilePath
			url := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", t.bot.Token, file.FilePath)
			resp, err := http.Get(url)
			var data []byte
			data, err = io.ReadAll(resp.Body)
			t.note.Upload(ctx, strings.Replace(nameTmp, "/", "", -1), data, int(update.Message.From.ID), t.state.Lat, t.state.Lon)
		}
	}
	//}()
	err := <-errChan
	return err
}

func (t *TelegramBot) handleMessage(msg *tgbotapi.Message) {
	if msg.IsCommand() {
		switch msg.Command() {
		case "start":
			t.sendMainMenu(msg.Chat.ID)
		}
	}
}

func (t *TelegramBot) sendMainMenu(chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "Choose an option:")

	// Create inline keyboard
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Add Note", "addNote"),
			//tgbotapi.NewInlineKeyboardButtonData("Option 2", "option2"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Help", "help"),
		),
	)

	msg.ReplyMarkup = keyboard
	t.bot.Send(msg)
}

func (t *TelegramBot) handleCallback(callback *tgbotapi.CallbackQuery) {
	// Acknowledge callback
	callbackCfg := tgbotapi.NewCallback(callback.ID, "")
	t.bot.Send(callbackCfg)

	// Handle button press
	switch callback.Data {
	case "addNote":
		t.state.State = ToSendLocation
		msg := tgbotapi.NewMessage(callback.Message.Chat.ID, "Send the item location first. Hower or press clip button")
		t.bot.Send(msg)
	case "help":
		t.sendHelp(callback.Message.Chat.ID)
	}
}

func (t *TelegramBot) sendHelp(chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "Help information goes here")
	t.bot.Send(msg)
}
