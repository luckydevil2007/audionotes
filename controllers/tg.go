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
	. "github.com/luckydevil2007/audionotes/entities"
	"github.com/luckydevil2007/audionotes/usecases"
)

type TelegramBot struct {
	bot      *tgbotapi.BotAPI // Экземпляр бота API Telegram
	chatID   int64            // ID чата, куда будут отправляться сообщения
	vicinity int64
	//checkAuth *usecases.AuthUseCase
	note        *usecases.NoteUseCase
	currentTour *usecases.Excursion
	state       map[int64]*InternalState
	//repo      *repositories.Repository
	//producer  *producers.EventProducer
}

func NewTelegramBot(token string, note *usecases.NoteUseCase, tour *usecases.Excursion) (*TelegramBot, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	var state = make(map[int64]*InternalState)
	//state[0].Set = make(map[State]State)
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

func (t *TelegramBot) sendAudioNote(chatId int64, note *Note) (tgbotapi.Message, error) {
	file := tgbotapi.FileBytes{
		Name:  note.Title,
		Bytes: note.Data,
	}

	audioConfig := tgbotapi.NewVoice(chatId, file)
	return t.bot.Send(audioConfig)
}

func (t *TelegramBot) Run(ctx context.Context) error {
	u := tgbotapi.NewUpdate(0)
	updates := t.bot.GetUpdatesChan(u)
	errChan := make(chan error)
	//go func() {
	for update := range updates {
		if update.CallbackQuery != nil { // Handle button presses
			t.handleCallback(ctx, update.CallbackQuery, update.CallbackQuery.Message.Chat.ID)
		}
		if update.Message == nil {
			continue
		}
		currentState := t.state[update.Message.Chat.ID]
		if currentState != nil && len(currentState.Set) > 0 {
			t.onCommand(ctx, update.Message)
			continue
		}

		// Handle /start command
		if update.Message.IsCommand() && update.Message.Command() == "start" {
			//	msg := tgbotapi.NewMessage(update.Message.From.ID, "Welcome! Use /newtour to create a tour.")
			t.sendMainMenu(update.Message.Chat.ID)
		}

		// Handle location sharing
		if update.Message.Location != nil {
			lat := float64(update.Message.Location.Latitude)
			lon := float64(update.Message.Location.Longitude)
			/*	if currentState.State == ToSendLocation {
				currentState.Lat = lat
				currentState.Lon = lon
				currentState.State = ToSendVoice
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Now record your voice message or send an audio OGG file")
				t.bot.Send(msg)
				continue
			}*/
			// Check if near a tour point (pseudo-c
			note, err := t.note.OpenNearest(ctx, lat, lon, 1.0)

			if err == nil {
				t.sendAudioNote(update.Message.Chat.ID, note)
				file := tgbotapi.FileBytes{
					Name:  note.Title,
					Bytes: note.Data,
				}

				audioConfig := tgbotapi.NewVoice(update.Message.Chat.ID, file)

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

func (t *TelegramBot) addAudioNote(ctx context.Context, msg *tgbotapi.Message) (note *Note, err error) {
	currentState := t.state[msg.Chat.ID]
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
	note = &Note{
		Title: strings.Replace(nameTmp, "/", "", -1),
		Path:  strings.Replace(nameTmp, "/", "", -1),
		Owner: int(msg.From.ID),
		Data:  data,
		Lat:   currentState.Lat,
		Lon:   currentState.Lon,
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
	currentState := t.getState(msg.Chat.ID)
	if currentState.Set[CreateTour] != 0 {
		t.createTour(ctx, msg)
		return
	}
	if currentState.Set[TakeTour] != 0 {
		t.takeTour(ctx, msg)
		return
	}
}

func (t *TelegramBot) takeTour(ctx context.Context, msg *tgbotapi.Message) error {
	currentState := t.state[msg.Chat.ID]
	if currentState.Set[TakeTour] == Start {
		currentState.Set[TakeTour] = ToSendLocation
		msg := tgbotapi.NewMessage(msg.Chat.ID, "Choose radius around you")
		msg.ReplyMarkup = t.getSelectRadiusKeyboard()
		t.bot.Send(msg)
		return nil
	}
	if currentState.Set[TakeTour] == ToSendLocation {
		if msg.Location == nil {
			msg := tgbotapi.NewMessage(msg.Chat.ID, "location is not set")
			t.bot.Send(msg)
			return nil
		}
		currentState.Lat = float64(msg.Location.Latitude)
		currentState.Lon = float64(msg.Location.Longitude)
		pathes, err := t.currentTour.Search(ctx, currentState.Lat, currentState.Lon, (float64)(currentState.RadiusM)/1000)

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
	if currentState.Set[TakeTour] == ToStartPointTour {
		path, err := t.currentTour.Load(ctx, currentState.CurrentTourID)
		if err != nil {
			return err
		}
		t.sendCurrentNote(ctx, msg, path.Head)

	}
	if currentState.Set[TakeTour] == ToNextPointTour {
		t.currentTour.NextNote(ctx)
		t.sendAudioNote(msg.Chat.ID, t.currentTour.Curr)
	}
	if currentState.Set[TakeTour] == ToPrevPointTour {

	}
	if currentState.Set[TakeTour] == ToCurrPointTour {
		t.sendAudioNote(msg.Chat.ID, t.currentTour.Curr)
	}
	return nil
}

func (t *TelegramBot) playCurrentNote(ctx context.Context, msg *tgbotapi.Message) error {
	note, err := t.note.Open(ctx, t.currentTour.Curr)
	if err == nil {
		t.sendAudioNote(msg.Chat.ID, note)
	}
	return nil
}

func (t *TelegramBot) sendCurrentNote(ctx context.Context, msg *tgbotapi.Message, note *entities.Note) error {
	messageText := note.Title
	msgLoc := tgbotapi.NewLocation(msg.Chat.ID, note.Lat, note.Lon)

	t.bot.Send(msgLoc)
	msgText := tgbotapi.NewMessage(msg.Chat.ID, messageText)
	t.bot.Send(msgText)
	t.sendAudioNote(msg.Chat.ID, note)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Play", "playCurrentNote"),
			tgbotapi.NewInlineKeyboardButtonData("Next", "toNextNote"),
		),
	)
	msgKeyboard := tgbotapi.NewMessage(msg.Chat.ID, messageText)
	msgKeyboard.ReplyMarkup = keyboard
	t.bot.Send(msgKeyboard)
	return nil
}

func (t *TelegramBot) toNextNote(ctx context.Context, msg *tgbotapi.Message) error {
	if t.currentTour.HasNext() {
		return nil
	}
	t.currentTour.NextNote(ctx)
	return t.sendCurrentNote(ctx, msg, t.currentTour.Curr)
}

func (t *TelegramBot) createTour(ctx context.Context, msg *tgbotapi.Message) error {
	currentState := t.state[msg.Chat.ID]
	if currentState.Set[CreateTour] == AddTourName {
		path := t.currentTour.CreatePath(ctx, msg.Text, int(msg.From.ID))
		err := t.currentTour.Save(ctx, path)
		if err != nil {
			delete(currentState.Set, CreateTour)
			return err
		}
		currentState.Set[CreateTour] = ToSendLocation
		msgConf := tgbotapi.NewMessage(msg.Chat.ID, "Pick the location")
		t.bot.Send(msgConf)
		return nil
	}
	if currentState.Set[CreateTour] == ToSendVoice {
		note, err := t.addAudioNote(ctx, msg)
		if err == nil {
			t.currentTour.AddAndUpload(ctx, note.Title, note.Data, note.Owner, note.Lat, note.Lon)
			currentState.Set[CreateTour] = ToSendLocation
		}
		t.sendAddNoteToTour(msg.From.ID)
		return err
	}
	if currentState.Set[CreateTour] == ToSendLocation {
		if msg.Location == nil {
			panic("location is not set")
		}
		currentState.Lat = float64(msg.Location.Latitude)
		currentState.Lon = float64(msg.Location.Longitude)
		currentState.Set[CreateTour] = ToSendVoice
		return nil
	}
	msgConf := tgbotapi.NewMessage(msg.Chat.ID, "Type the tour name")
	currentState.Set[CreateTour] = AddTourName
	t.bot.Send(msgConf)
	return nil
}

func (t *TelegramBot) finishTour(chatID int64) {
	currentState := t.state[chatID]
	err := t.currentTour.Finish( /*ctx*/ )
	if err != nil {
		msg := tgbotapi.NewMessage(chatID, "Error! The tour has not been saved")
		currentState.Set[CreateTour] = Nothing
		t.bot.Send(msg)
	}
	msg := tgbotapi.NewMessage(chatID, "The tour has been saved")
	currentState.Set[CreateTour] = Nothing
	t.bot.Send(msg)
}

func (t *TelegramBot) getSelectRadiusKeyboard() tgbotapi.ReplyKeyboardMarkup {
	/*
		return tgbotapi.NewInlineKeyboardMarkup(
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

func (t *TelegramBot) getState(chatID int64) *entities.InternalState {
	currentState, ok := t.state[chatID]
	if !ok {
		currentState = NewInternalState()
		currentState.Set = map[State]State{Nothing: Nothing}
		t.state[chatID] = currentState
	}
	return t.state[chatID]
}
func (t *TelegramBot) handleCallback(ctx context.Context, callback *tgbotapi.CallbackQuery, chatID int64) {
	// Acknowledge callback
	callbackCfg := tgbotapi.NewCallback(callback.ID, "")
	t.bot.Send(callbackCfg)
	// create state if don't have yet
	currentState := t.getState(chatID)
	// Handle button press
	num, err := strconv.Atoi(callback.Data)
	if err == nil {
		_, x := currentState.Set[TakeTour]
		if !x {
			return
		}
		currentState.CurrentTourID = num
		currentState.Set[TakeTour] = ToStartPointTour
		t.onCommand(ctx, callback.Message)
		return
	}
	switch callback.Data {
	case "takeTour":
		currentState.Set[TakeTour] = Start
		currentState.RadiusM = 1000

		t.onCommand(ctx, callback.Message)
	case "addNote":
		currentState.State = ToSendLocation

		msg := tgbotapi.NewMessage(callback.Message.Chat.ID, "Send the item location first. Hower or press clip button")
		t.bot.Send(msg)
	case "newTour":
		currentState.Set[CreateTour] = Start

		t.onCommand(ctx, callback.Message)
	case "finishTour":
		t.finishTour(callback.Message.Chat.ID)
	case "help":
		t.sendHelp(callback.Message.Chat.ID)
	case "500m":
		currentState.RadiusM = 500
	case "1km":
		currentState.RadiusM = 1000
	case "10km":
		currentState.RadiusM = 10000
	case "5km":
		currentState.RadiusM = 5000
	case "2km":
		currentState.RadiusM = 2000
	case "playCurrentNote":
		t.playCurrentNote(ctx, callback.Message)
	case "toNextNote":
		t.toNextNote(ctx, callback.Message)
	}
}

func (t *TelegramBot) sendHelp(chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "Help information goes here")
	t.bot.Send(msg)
}
