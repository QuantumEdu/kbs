package domain

// Project is a logical container for vault entries and series.
type Project struct {
	ID          string
	Name        string
	Description string
	Active      bool
}
