package domain

import "time"

type Workflow struct {
	ID          string
	Name        string
	Slug        string
	Description string
	Status      Status
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type WorkflowStep struct {
	ID             string
	WorkflowID     string
	OrderIndex     int
	Title          string
	Instruction    string
	Required       bool
	ExpectedOutput string
}
