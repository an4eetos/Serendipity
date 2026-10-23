package domain

import (
	"errors"
	"testing"
)

func ptr[T any](v T) *T { return &v }

func TestPatchValidate(t *testing.T) {
	const owner = "user-1"
	tests := []struct {
		name  string
		patch Patch
		want  error
	}{
		{"empty", Patch{}, nil},
		{"progress in range", Patch{Progress: ptr(0.5)}, nil},
		{"progress below 0", Patch{Progress: ptr(-0.1)}, ErrProgressRange},
		{"progress above 1", Patch{Progress: ptr(1.1)}, ErrProgressRange},
		{"valid status", Patch{Status: ptr(StatusFinished)}, nil},
		{"unknown status", Patch{Status: ptr(ReadingStatus("paused"))}, ErrInvalidStatus},
		{"path without type", Patch{FilePath: ptr(owner + "/a.epub")}, ErrFileFieldsPaired},
		{"type without path", Patch{FileType: ptr(FileEPUB)}, ErrFileFieldsPaired},
		{"own file", Patch{FilePath: ptr(owner + "/a.epub"), FileType: ptr(FileEPUB), OwnershipConfirmed: true}, nil},
		{"own file without the ownership confirmation", Patch{FilePath: ptr(owner + "/a.epub"), FileType: ptr(FileEPUB)}, ErrOwnershipUnconfirmed},
		{"someone else's folder", Patch{FilePath: ptr("user-2/a.epub"), FileType: ptr(FileEPUB)}, ErrForeignFile},
		{"someone else's folder, confirmed", Patch{FilePath: ptr("user-2/a.epub"), FileType: ptr(FileEPUB), OwnershipConfirmed: true}, ErrForeignFile},
		{"path traversal", Patch{FilePath: ptr(owner + "/../user-2/a.pdf"), FileType: ptr(FilePDF)}, ErrForeignFile},
		{"unsupported type", Patch{FilePath: ptr(owner + "/a.mobi"), FileType: ptr(FileType("mobi"))}, ErrUnsupportedFile},
		{"pairing is checked before ownership", Patch{FilePath: ptr(owner + "/a.epub")}, ErrFileFieldsPaired},
		{"no file needs no confirmation", Patch{Progress: ptr(0.5)}, nil},
		{"a confirmation without a file", Patch{OwnershipConfirmed: true}, nil},
		{"first failing rule wins", Patch{Progress: ptr(2.0), Status: ptr(ReadingStatus("x"))}, ErrProgressRange},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.patch.Validate(owner); !errors.Is(got, tt.want) {
				t.Errorf("Validate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheckOwnershipConfirmed(t *testing.T) {
	if err := CheckOwnershipConfirmed(false); !errors.Is(err, ErrOwnershipUnconfirmed) {
		t.Errorf("unconfirmed: err = %v, want ErrOwnershipUnconfirmed", err)
	}
	if err := CheckOwnershipConfirmed(true); err != nil {
		t.Errorf("confirmed: err = %v", err)
	}
}

func TestUploadedFile(t *testing.T) {
	if _, _, err := (Item{}).UploadedFile(); !errors.Is(err, ErrNoFile) {
		t.Errorf("no file: err = %v", err)
	}
	path, ft, err := Item{FilePath: ptr("u/a.pdf"), FileType: ptr(FilePDF)}.UploadedFile()
	if err != nil || path != "u/a.pdf" || ft != FilePDF {
		t.Errorf("UploadedFile() = %q, %q, %v", path, ft, err)
	}
}
