package domain

import (
	"time"
)

type ProjectState string

const (
	StateCreating ProjectState = "creating"
	StateInactive ProjectState = "inactive"
	StateActive   ProjectState = "active"
	StateDeleting ProjectState = "deleting"
	StateDeleted  ProjectState = "deleted"
	StateWiped    ProjectState = "wiped"
)

type Project struct {
	Name  string
	ID    string
	State ProjectState

	CreatedAt time.Time

	UpdatedAt time.Time
}

type Scanner interface {
	Scan(dest ...any) error
}

func ScanProject(s Scanner, ptrProj *Project) error {
	return s.Scan(&ptrProj.ID, &ptrProj.Name, &ptrProj.State, &ptrProj.CreatedAt, &ptrProj.UpdatedAt)
}

func (p ProjectState) IsValid() bool {
	switch p {
	case StateCreating,
		StateInactive,
		StateActive,
		StateDeleting,
		StateDeleted,
		StateWiped:
		return true
	default:
		return false
	}
}
