package usecases

import (
	"context"

	"github.com/luckydevil2007/audionotes/adapters/repositories"
	"github.com/luckydevil2007/audionotes/entities"
)

type ExcursionService interface {
	Play(transform entities.Note, data []byte) ([]byte, error)
}

type IStorage interface {
	Save(ctx context.Context, image *entities.Note) error
	Delete(ctx context.Context, image *entities.Note) error
	Open(ctx context.Context, path string) ([]byte, error)
}

type ICacheRepository interface {
	SelectUserNotes(ctx context.Context, user *entities.User) ([]entities.Note, error)
}

type NoteUseCase struct {
	repo           *repositories.Repository // вынести в интерфейс репозиторий
	cacheImageRepo ICacheRepository
	fileStorage    IStorage
}

func NewNoteUseCase(repo *repositories.Repository,
	fileStorage IStorage) *NoteUseCase {
	return &NoteUseCase{
		repo:        repo,
		fileStorage: fileStorage,
	}
}

func (uc *NoteUseCase) Upload(ctx context.Context, name string, data []byte, ownerID int, lat, lon float64) error {
	note := &entities.Note{
		Title: name,
		Path:  name,
		Owner: ownerID,
		Data:  data,
		Lat:   lat,
		Lon:   lon,
	}
	if err := uc.fileStorage.Save(ctx, note); err != nil {
		return err
	}

	return uc.repo.SaveNote(ctx, note)
}

func (uc *NoteUseCase) ListUsers(ctx context.Context, user entities.User) (notes []entities.Note, err error) {

	notes, err = uc.repo.SelectUserNotes(ctx, &user) //cacheImagerRepo
	if err != nil {
		return nil, err
	}

	return notes, nil

}

func (uc *NoteUseCase) ListNearest(ctx context.Context, lat float64, lon float64, radius float64) (notes []entities.Note, err error) {
	notes, err = uc.repo.ClosestNotes(ctx, lat, lon, radius)
	if err != nil {
		return nil, err
	}
	return notes, nil
}

func (uc *NoteUseCase) Open(ctx context.Context, note *entities.Note) (err error) {
	err = uc.repo.OpenNote(ctx, note)
	if err != nil {
		return err
	}
	uc.fileStorage.Open(ctx, note.Path)
	return err
}

func (uc *NoteUseCase) OpenNearest(ctx context.Context, lat float64, lon float64, radius float64) (note *entities.Note, err error) {
	notes, err := uc.ListNearest(ctx, lat, lon, radius)
	if err != nil {
		return nil, err
	}
	notes[0].Data, err = uc.fileStorage.Open(ctx, notes[0].Path)
	return &notes[0], err
}

func (uc *NoteUseCase) Delete(ctx context.Context, image *entities.Note) error {
	if err := uc.fileStorage.Delete(ctx, image); err != nil {
		return err
	}
	return uc.repo.DeleteNote(ctx, image.ID)
}
