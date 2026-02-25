package entities

type Note struct {
	ID        int
	Title     string
	Path      string
	Owner     int
	Data      []byte
	Lat       float64
	Lon       float64
	Excursion int
	Next      *Note
	Prev      *Note
}

type Path struct {
	ID    int
	Title string
	Owner int
	Head  *Note
}

type Excursion struct {
	Path *Path
	Curr *Note
	Next *Note
}
