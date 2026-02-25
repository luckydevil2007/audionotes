package repositories

import (
	"context"
	"database/sql"
	"math"

	"github.com/luckydevil2007/audionotes/entities"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}
func (r *Repository) SelectUserNotes(ctx context.Context, user *entities.User) ([]entities.Note, error) {
	content, err := r.db.QueryContext(ctx,
		`SELECT title, path, id FROM notes WHERE owner_id = $1`, user.ID)
	var res []entities.Note
	for content.Next() {
		var tmp entities.Note
		content.Scan(&tmp.Title, &tmp.Path, &tmp.Owner)
		res = append(res, tmp)
	}

	return res, err
}

func (r *Repository) SelectUserPathes(ctx context.Context, user *entities.User) ([]entities.Path, error) {
	content, err := r.db.QueryContext(ctx,
		`SELECT title, owner_id FROM pthes WHERE owner_id = $1`, user.ID)
	var res []entities.Path
	for content.Next() {
		var tmp entities.Path
		content.Scan(&tmp.Title, &tmp.Owner)
		res = append(res, tmp)
	}

	return res, err
}

func (r *Repository) SaveNote(ctx context.Context, note *entities.Note) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO notes (note_title, note_path, owner_id, lat, lon, excursion_id) VALUES ($1, $2, $3, $4, $5, $6)`,
		note.Title, note.Path, note.Owner, note.Lat, note.Lon, note.Excursion)
	return err
}

func (r *Repository) DeleteNote(ctx context.Context, id int) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE * FROM notes WHERE id = $1`,
		id)
	return err
}

func (r *Repository) OpenNote(ctx context.Context, note *entities.Note) error {
	err := r.db.QueryRowContext(ctx,
		`SELECT title, path, owner_id FROM notes WHERE ID = $1`, note.ID).Scan(
		&note.Title, &note.Path, &note.Owner)
	return err
}

func (r *Repository) ClosestNotes(ctx context.Context, lat float64, lon float64, radius float64) ([]entities.Note, error) {

	kmGradLon := float64(40000.0 * math.Cos(lat/180*math.Pi) / 360)
	kmGradLat := float64(40000.0 / 360.0)
	min_lat := lat - radius/kmGradLat
	max_lat := lat + radius/kmGradLat
	min_lon := lon - radius/kmGradLon
	max_lon := lon + radius/kmGradLon
	/*var tableExists bool
	err := r.db.QueryRow(`
		SELECT EXISTS (
			SELECT FROM information_schema.tables
			WHERE table_schema = 'public'
			AND table_name = 'notes'
		)`).Scan(&tableExists)*/
	rows, err := r.db.Query(
		`SELECT * FROM notes WHERE
		 lat BETWEEN $1 AND $2
	     AND lon BETWEEN $3 AND $4`,
		min_lat, max_lat, min_lon, max_lon)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var locations []entities.Note
	for rows.Next() {
		var loc entities.Note
		var tour_id sql.NullInt32
		err := rows.Scan(&loc.ID, &loc.Title, &loc.Owner, &loc.Path, &loc.Lat, &loc.Lon, &tour_id)
		if err != nil {
			return nil, err
		}
		if tour_id.Valid {
			loc.Excursion = int(tour_id.Int32)
		}
		locations = append(locations, loc)
	}

	return locations, nil
}

func (r *Repository) OpenPath(ctx context.Context, path *entities.Path) (p *entities.Path, err error) {
	var ownerId sql.NullInt64

	err = r.db.QueryRowContext(ctx,
		`SELECT path_title, owner_id FROM pathes WHERE ID = $1`, path.ID).Scan(
		&path.Title, &ownerId)

	if ownerId.Valid {
		path.Owner = int(ownerId.Int64)
	}

	rows, err := r.db.Query(
		`SELECT * FROM notes WHERE excursion_id = $1`, path.ID)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var notes []entities.Note
	for rows.Next() {
		var n entities.Note
		var tour_id sql.NullInt32
		err := rows.Scan(&n.ID, &n.Title, &n.Owner, &n.Path, &n.Lat, &n.Lon, &tour_id)
		if err != nil {
			return nil, err
		}
		if tour_id.Valid {
			n.Excursion = int(tour_id.Int32)
		}
		notes = append(notes, n)
	}
	if rows.Err() != nil || len(notes) == 0 {
		return nil, rows.Err()
	}
	path.Head = &notes[0]
	cur := path.Head
	for i := 1; i < len(notes); i++ {
		cur.Next = &notes[1]
	}
	return path, err
}

func (r *Repository) SavePath(ctx context.Context, path *entities.Path) error {
	var id int
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO pathes (path_title) VALUES ($1) RETURNING id`,
		path.Title).Scan(&id)
	path.ID = id
	return err
}
