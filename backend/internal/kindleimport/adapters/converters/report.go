// Package converters maps Kindle import results to HTTP responses.
package converters

import "github.com/an4eetos/serendipity/backend/internal/kindleimport/domain"

type ReportResponse struct {
	Books          int `json:"books"`
	Imported       int `json:"imported"`
	Duplicates     int `json:"duplicates"`
	SkippedTooLong int `json:"skipped_too_long"`
	OrphanNotes    int `json:"orphan_notes"`
}

func ToReportResponse(r domain.Report) ReportResponse {
	return ReportResponse{
		Books:          r.Books,
		Imported:       r.Imported,
		Duplicates:     r.Duplicates,
		SkippedTooLong: r.SkippedTooLong,
		OrphanNotes:    r.OrphanNotes,
	}
}
