package model

import (
	"time"

	"github.com/google/uuid"
)

type Image struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	Name       string
	MimeType   string
	UploadedAt time.Time
	Size       int64
	SizeLarge  int64
	SizeMedium int64
	SizeSmall  int64
	Status     string
}
