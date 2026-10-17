package usecase

import (
	"context"

	"github.com/an4eetos/serendipity/backend/internal/library/domain"
	"github.com/an4eetos/serendipity/backend/internal/library/ports"
)

// GetFileLink hands the reader a short-lived link to their own uploaded copy.
type GetFileLink struct {
	library ports.LibraryRepository
	files   ports.FileStorage
}

func NewGetFileLink(library ports.LibraryRepository, files ports.FileStorage) *GetFileLink {
	return &GetFileLink{library: library, files: files}
}

func (uc *GetFileLink) Execute(ctx context.Context, userID, id string) (domain.FileLink, error) {
	item, err := uc.library.Get(ctx, userID, id)
	if err != nil {
		return domain.FileLink{}, err
	}
	path, fileType, err := item.UploadedFile()
	if err != nil {
		return domain.FileLink{}, err
	}
	url, err := uc.files.SignedURL(ctx, path, domain.FileLinkTTL)
	if err != nil {
		return domain.FileLink{}, err
	}
	return domain.FileLink{URL: url, FileType: fileType}, nil
}
