package usecases

import (
	"context"

	"github.com/luckydevil2007/audionotes/adapters/repositories"
	"github.com/luckydevil2007/audionotes/entities"
)

type INoteService interface {
	Upload(ctx context.Context, name string, data []byte, ownerID int, lat, lon float64) (note *entities.Note, err error)
	UploadNote(ctx context.Context, note *entities.Note) (err error)
	ListUsers(ctx context.Context, user entities.User) (notes []entities.Note, err error)
	ListNearest(ctx context.Context, lat float64, lon float64, radius float64) (notes []entities.Note, err error)
	Open(ctx context.Context, note *entities.Note) (n *entities.Note, err error)
	OpenNearest(ctx context.Context, lat float64, lon float64, radius float64) (note *entities.Note, err error)
	Delete(ctx context.Context, note *entities.Note) error
}

type Excursion struct {
	path        *entities.Path
	curr        *entities.Note
	next        *entities.Note
	repo        *repositories.Repository
	noteService INoteService
}

func NewExcursion(repo *repositories.Repository, noteService INoteService) *Excursion {
	return &Excursion{path: nil, curr: nil, next: nil, repo: repo, noteService: noteService}
}

func (e *Excursion) CreatePath(ctx context.Context, name string, ownerID int) *entities.Path {
	path := &entities.Path{
		Title: name,
		Owner: ownerID,
	}
	note := &entities.Note{
		ID:    -1,
		Title: "Dummy",
		Path:  path.Title,
	}
	path.Head = note
	e.curr = path.Head
	e.path = path
	return e.path
}

func (e *Excursion) AddNote(ctx context.Context, note *entities.Note) error {
	note.Excursion = e.path.ID
	note.Prev = e.curr
	e.curr.Next = note
	return nil
}

func (e *Excursion) AddAndUpload(ctx context.Context, title string, data []byte, ownerID int, lat, lon float64) error {
	note := &entities.Note{
		Title:     title,
		Path:      title,
		Owner:     ownerID,
		Data:      data,
		Lat:       lat,
		Lon:       lon,
		Excursion: e.path.ID,
	}
	err := e.noteService.UploadNote(ctx, note)
	if err != nil {
		return err
	}

	return e.AddNote(ctx, note)
}

func (e *Excursion) Load(ctx context.Context, id int) (path *entities.Path, err error) {
	path = &entities.Path{ID: id}
	return e.repo.OpenPath(ctx, path)
}

func (e *Excursion) NextNote(ctx context.Context) (note *entities.Note, err error) {
	return e.noteService.Open(ctx, e.next)

}

func (e *Excursion) Search(ctx context.Context, lat, lon float64, radius float64) (pathes []entities.Path, err error) {
	notes, err := e.noteService.ListNearest(ctx, lat, lon, radius)
	if err != nil {
		return nil, err
	}
	var excursionIds []int
	for n := range len(notes) {
		excursionIds = append(excursionIds, notes[n].Excursion)
	}

	for i := range excursionIds {
		p, err := e.Load(ctx, excursionIds[i])
		if err == nil {
			pathes = append(pathes, *p)
		}
	}

	return pathes, err
}

func (e *Excursion) Save(ctx context.Context, path *entities.Path) error {
	return e.repo.SavePath(ctx, path)
}

func (e *Excursion) Remove(ctx context.Context) error {
	return nil
}

func (e *Excursion) Finish( /*ctx context.Context*/ ) error {
	e.path = nil
	e.curr = nil
	e.next = nil
	return nil
}
