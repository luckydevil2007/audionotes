package controllers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/luckydevil2007/audionotes/entities"
	"github.com/luckydevil2007/audionotes/usecases"
)

type State int

const (
	Nothing State = iota
	Start
	ToSendVoice
	ToSendLocation
	CreateTour
	AddTourName
	FinalizeTour
	TakeTour
)

type InternalState struct {
	State    State
	Lat, Lon float64
	RadiusM  int64
	Set      map[State]State
}

type TelegramBot struct {
	bot      *tgbotapi.BotAPI // Экземпляр бота API Telegram
	chatID   int64            // ID чата, куда будут отправляться сообщения
	vicinity int64
	//checkAuth *usecases.AuthUseCase
	note        *usecases.NoteUseCase
	currentTour *usecases.Excursion
	state       InternalState
	//repo      *repositories.Repository
	//producer  *producers.EventProducer
}

func NewTelegramBot(token string, note *usecases.NoteUseCase, tour *usecases.Excursion) (*TelegramBot, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	var state InternalState
	state.Set = make(map[State]State)
	// Возвращаем инициализированный адаптер с ботом и ID чата
	return &TelegramBot{bot: bot, note: note, currentTour: tour, state: state}, nil
}

func (t *TelegramBot) Test(ctx context.Context) error {
	note, err := t.note.OpenNearest(ctx, 60.0, 30.0, 1000.0)
	if err != nil {
		return err
	}
	if len(note.Data) == 0 {
		return fmt.Errorf("data==0")
	}

	_, err = t.currentTour.Search(ctx, 60.0, 30.0, 1000.0)
	if err != nil {
		return err
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
			t.handleCallback(ctx, update.CallbackQuery)
		}
		if update.Message == nil {
			continue
		}

		if len(t.state.Set) > 0 {
			t.onCommand(ctx, update.Message)
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
			} else {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, err.Error())
				t.bot.Send(msg)
			}
		}

		if update.Message.Voice != nil || update.Message.Audio != nil {
			note, err := t.addAudioNote(ctx, update.Message)
			if err == nil {
				t.note.UploadNote(ctx, note)
			} else {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, err.Error())
				t.bot.Send(msg)
			}
		}
	}
	//}()
	err := <-errChan
	return err
}

func (t *TelegramBot) addAudioNote(ctx context.Context, msg *tgbotapi.Message) (note *entities.Note, err error) {
	if msg.Voice == nil && msg.Audio == nil {
		panic("This is not an audio note")

	}
	var fileID string
	if msg.Voice != nil {
		fileID = msg.Voice.FileID
	} else {
		fileID = msg.Audio.FileID
	}
	fileConfig := tgbotapi.FileConfig{FileID: fileID}
	file, err := t.bot.GetFile(fileConfig)
	if err != nil {
		return nil, err
	}
	nameTmp := t.bot.Token + "/" + file.FilePath
	url := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", t.bot.Token, file.FilePath)
	resp, err := http.Get(url)
	var data []byte
	data, err = io.ReadAll(resp.Body)
	note = &entities.Note{
		Title: strings.Replace(nameTmp, "/", "", -1),
		Path:  strings.Replace(nameTmp, "/", "", -1),
		Owner: int(msg.From.ID),
		Data:  data,
		Lat:   t.state.Lat,
		Lon:   t.state.Lon,
	}
	return note, nil
}

func (t *TelegramBot) handleMessage(msg *tgbotapi.Message) {
	if msg.IsCommand() {
		switch msg.Command() {
		case "start":
			t.sendMainMenu(msg.Chat.ID)
		}
	}
}

func (t *TelegramBot) onCommand(ctx context.Context, msg *tgbotapi.Message) {
	if t.state.Set[CreateTour] != 0 {
		t.createTour(ctx, msg)
		return
	}
	if t.state.Set[TakeTour] != 0 {
		t.takeTour(ctx, msg)
		return
	}
}

func (t *TelegramBot) takeTour(ctx context.Context, msg *tgbotapi.Message) error {
	if t.state.Set[TakeTour] == Start {
		t.state.Set[TakeTour] = ToSendLocation
		msg := tgbotapi.NewMessage(msg.Chat.ID, "Choose radius around you")
		msg.ReplyMarkup = t.getSelectRadiusKeyboard()
		t.bot.Send(msg)
		return nil
	}
	if t.state.Set[TakeTour] == ToSendLocation {
		if msg.Location == nil {
			msg := tgbotapi.NewMessage(msg.Chat.ID, "location is not set")
			t.bot.Send(msg)
			return nil
		}
		t.state.Lat = float64(msg.Location.Latitude)
		t.state.Lon = float64(msg.Location.Longitude)
		pathes, err := t.currentTour.Search(ctx, t.state.Lat, t.state.Lon, (float64)(t.state.RadiusM)/1000)

		keys := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow())
		if len(pathes) == 0 {
			msg := tgbotapi.NewMessage(msg.Chat.ID, "No pathes found around you")
			t.bot.Send(msg)
			return nil
		}

		for i := range len(pathes) {
			keys.InlineKeyboard[0] = append(keys.InlineKeyboard[0], tgbotapi.NewInlineKeyboardButtonData(pathes[i].Title, strconv.Itoa(pathes[i].ID)))

		}
		msg := tgbotapi.NewMessage(msg.Chat.ID, "Choose path")
		msg.ReplyMarkup = keys
		t.bot.Send(msg)
		return err

	}
	return nil
}

func (t *TelegramBot) createTour(ctx context.Context, msg *tgbotapi.Message) error {
	if t.state.Set[CreateTour] == AddTourName {
		path := t.currentTour.CreatePath(ctx, msg.Text, int(msg.Chat.ID))
		err := t.currentTour.Save(ctx, path)
		if err != nil {
			delete(t.state.Set, CreateTour)
			return err
		}
		t.state.Set[CreateTour] = ToSendLocation
		msgConf := tgbotapi.NewMessage(msg.Chat.ID, "Pick the location")
		t.bot.Send(msgConf)
		return nil
	}
	if t.state.Set[CreateTour] == ToSendVoice {
		note, err := t.addAudioNote(ctx, msg)
		if err == nil {
			t.currentTour.AddAndUpload(ctx, note.Title, note.Data, note.Owner, note.Lat, note.Lon)
			t.state.Set[CreateTour] = ToSendLocation
		}
		t.sendAddNoteToTour(msg.Chat.ID)
		return err
	}
	if t.state.Set[CreateTour] == ToSendLocation {
		if msg.Location == nil {
			panic("location is not set")
		}
		t.state.Lat = float64(msg.Location.Latitude)
		t.state.Lon = float64(msg.Location.Longitude)
		t.state.Set[CreateTour] = ToSendVoice
		return nil
	}
	msgConf := tgbotapi.NewMessage(msg.Chat.ID, "Type the tour name")
	t.state.Set[CreateTour] = AddTourName
	t.bot.Send(msgConf)
	return nil
}

func (t *TelegramBot) finishTour(chatID int64) {
	err := t.currentTour.Finish( /*ctx*/ )
	if err != nil {
		msg := tgbotapi.NewMessage(chatID, "Error! The tour has not been saved")
		t.state.Set[CreateTour] = Nothing
		t.bot.Send(msg)
	}
	msg := tgbotapi.NewMessage(chatID, "The tour has been saved")
	t.state.Set[CreateTour] = Nothing
	t.bot.Send(msg)
}

func (t *TelegramBot) getSelectRadiusKeyboard() tgbotapi.ReplyKeyboardMarkup {
	t.state.Set[TakeTour] = ToSendLocation
	/*return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewKeyboardButtonLocation("500m"),
			tgbotapi.NewInlineKeyboardButtonData("1km", "1km"),
			tgbotapi.NewInlineKeyboardButtonData("2km", "2km"),
			tgbotapi.NewInlineKeyboardButtonData("5km", "5km"),
			tgbotapi.NewInlineKeyboardButtonData("10km", "10km"),
			tgbotapi.NewInlineKeyboardButtonData("Map", "Map"),
		),
	)*/
	//location500m := tgbotapi.NewInlineKeyboardButtonData("500m","500m")
	location500m := tgbotapi.KeyboardButton{
		Text:            "Current Location",
		RequestLocation: true,
	}
	location2km := tgbotapi.NewKeyboardButtonLocation("1km")
	location10km := tgbotapi.NewKeyboardButtonLocation("10km")
	return tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(location500m, location2km, location10km),
	)
}

func (t *TelegramBot) sendMainMenu(chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "Choose an option:")

	// Create inline keyboard
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Take a tour", "takeTour"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Add tour", "newTour"),
			tgbotapi.NewInlineKeyboardButtonData("Add Note", "addNote"),
		),

		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Help", "help"),
		),
	)

	msg.ReplyMarkup = keyboard
	t.bot.Send(msg)
}

func (t *TelegramBot) sendAddNoteToTour(chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "Choose an option:")

	// Create inline keyboard
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Finalize", "finishTour"),
			tgbotapi.NewInlineKeyboardButtonData("Add Note", "addNote"),
		),
	)

	msg.ReplyMarkup = keyboard
	t.bot.Send(msg)
}

func (t *TelegramBot) handleCallback(ctx context.Context, callback *tgbotapi.CallbackQuery) {
	// Acknowledge callback
	callbackCfg := tgbotapi.NewCallback(callback.ID, "")
	t.bot.Send(callbackCfg)

	// Handle button press
	switch callback.Data {
	case "takeTour":
		t.state.Set[TakeTour] = Start
		t.state.RadiusM = 1000
		t.onCommand(ctx, callback.Message)
	case "addNote":
		t.state.State = ToSendLocation
		msg := tgbotapi.NewMessage(callback.Message.Chat.ID, "Send the item location first. Hower or press clip button")
		t.bot.Send(msg)
	case "newTour":
		t.state.Set[CreateTour] = Start
		t.onCommand(ctx, callback.Message)
	case "finishTour":
		t.finishTour(callback.Message.Chat.ID)
	case "help":
		t.sendHelp(callback.Message.Chat.ID)
	case "500m":
		t.state.RadiusM = 500
	case "1km":
		t.state.RadiusM = 1000
	case "10km":
		t.state.RadiusM = 10000
	case "5km":
		t.state.RadiusM = 5000
	case "2km":
		t.state.RadiusM = 2000
	}
}

func (t *TelegramBot) sendHelp(chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "Help information goes here")
	t.bot.Send(msg)
}
