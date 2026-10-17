// Package domain holds the rules of a reader's library: the works they read,
// how far along they are, and the private copy they may upload of each.
package domain

import (
	"strings"
	"time"

	catalog "github.com/an4eetos/serendipity/backend/internal/catalog/domain"
	kernel "github.com/an4eetos/serendipity/backend/internal/kernel/domain"
)

type ReadingStatus string

const (
	StatusReading   ReadingStatus = "reading"
	StatusFinished  ReadingStatus = "finished"
	StatusAbandoned ReadingStatus = "abandoned"
)

func (s ReadingStatus) Valid() bool {
	switch s {
	case StatusReading, StatusFinished, StatusAbandoned:
		return true
	}
	return false
}

type FileType string

const (
	FileEPUB FileType = "epub"
	FilePDF  FileType = "pdf"
)

func (t FileType) Valid() bool {
	return t == FileEPUB || t == FilePDF
}

// FileLinkTTL is how long a download link to a reader's own file works.
const FileLinkTTL = 10 * time.Minute

var (
	ErrProgressRange    = kernel.Invalid("progress must be between 0 and 1")
	ErrInvalidStatus    = kernel.Invalid("invalid status")
	ErrFileFieldsPaired = kernel.Invalid("file_path and file_type go together")
	ErrForeignFile      = kernel.Invalid("file must be in your own folder")
	ErrUnsupportedFile  = kernel.Invalid("only EPUB and PDF files are supported")
	ErrNoFile           = kernel.NotFound("no file uploaded for this book")
)

// Item is a reader's copy of a work.
type Item struct {
	ID           string
	Work         catalog.Work
	FilePath     *string
	FileType     *FileType
	Progress     float64
	Status       ReadingStatus
	LastLocation *string
	QuoteCount   int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// UploadedFile returns where the reader's own copy is stored.
func (it Item) UploadedFile() (path string, fileType FileType, err error) {
	if it.FilePath == nil || it.FileType == nil {
		return "", "", ErrNoFile
	}
	return *it.FilePath, *it.FileType, nil
}

// FileLink is a short-lived download link to a reader's own copy.
type FileLink struct {
	URL      string
	FileType FileType
}

// Patch changes a library item; nil fields stay as they are.
type Patch struct {
	Progress     *float64
	Status       *ReadingStatus
	LastLocation *string
	FilePath     *string
	FileType     *FileType
}

// Validate checks a patch made by the item's owner.
func (p Patch) Validate(ownerID string) error {
	if p.Progress != nil && (*p.Progress < 0 || *p.Progress > 1) {
		return ErrProgressRange
	}
	if p.Status != nil && !p.Status.Valid() {
		return ErrInvalidStatus
	}
	if (p.FilePath == nil) != (p.FileType == nil) {
		return ErrFileFieldsPaired
	}
	if p.FilePath != nil {
		// Files live in the uploader's own folder of the private bucket.
		if !strings.HasPrefix(*p.FilePath, ownerID+"/") || strings.Contains(*p.FilePath, "..") {
			return ErrForeignFile
		}
		if !p.FileType.Valid() {
			return ErrUnsupportedFile
		}
	}
	return nil
}
