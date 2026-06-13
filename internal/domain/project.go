package domain

import "time"

type Project struct {
	ID          string
	Name        string
	Slug        string
	Description string
	Status      Status
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
