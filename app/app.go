package app

import (
	"context"
	"fmt"
	"net/http"

	_ "github.com/lib/pq"
	"github.com/luckydevil2007/audionotes/adapters/repositories"
	"github.com/luckydevil2007/audionotes/controllers"
	"github.com/luckydevil2007/audionotes/database"
	"github.com/luckydevil2007/audionotes/migrations"
	"github.com/luckydevil2007/audionotes/usecases"
)

func Hash(str, key string) string {

	return str
}

//controllers
//----http
//----другой способ дергания юзкейсов

func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}

func Run() {

	db := database.OpenPG()
	//clickhouseDb := database.OpenClickhouse()
	migrations.ApplyMigrationsPG(db)
	botToken := "8476210801:AAHVqsNsZ-KOW5Rl_W6zBeAQfb8to8MxQrs"

	useRepo := repositories.NewUserRepository(db)
	notesRepo := repositories.NewRepository(db)
	fileStorage := repositories.NewFileStorage(db, "storage")
	//s3Storage := s3storage.NewS3Storage("mybucket")
	authUsecase := usecases.NewAuthUseCase(useRepo)
	noteUsecase := usecases.NewNoteUseCase(notesRepo, fileStorage)
	pathUsecase := usecases.NewExcursion(notesRepo, noteUsecase)
	auth := controllers.NewAuthController(authUsecase)
	ctx := context.Background()
	auth.Authenticate(ctx, "user")
	// нужна ли аутентификация и authUsecase в других контроллерах когда есть AuthMiddleware
	c, err := controllers.NewTelegramBot(botToken, noteUsecase, pathUsecase)
	if err != nil {
		fmt.Println("Couldn't create bot instance")
		return
	}
	err = c.Test(ctx)

	if err != nil {
		fmt.Println(err.Error())
		return
	}
	if c.Run(ctx) != nil {
		fmt.Println("Couldn't run the bot")
	}
	//h := controllers.NewHttpController(imageRepo)
	//authController := controllers.NewAuthController(authUsecase)
	//TODO middleware for autorization

}

func mainHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello")
}
