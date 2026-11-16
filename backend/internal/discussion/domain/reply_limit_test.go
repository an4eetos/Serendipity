package domain

import (
	"errors"
	"testing"

	kernel "github.com/an4eetos/serendipity/backend/internal/kernel/domain"
)

func TestReplyLimitCheckRoom(t *testing.T) {
	for _, tt := range []struct {
		recent int
		want   error
	}{
		{0, nil},
		{59, nil},
		{60, ErrReplyLimit},
		{61, ErrReplyLimit},
	} {
		if err := ReplyLimit(60).CheckRoom(tt.recent); !errors.Is(err, tt.want) {
			t.Errorf("CheckRoom(%d) = %v, want %v", tt.recent, err, tt.want)
		}
	}
	if ErrReplyLimit.Kind() != kernel.KindRateLimited {
		t.Errorf("ErrReplyLimit kind = %v, want KindRateLimited", ErrReplyLimit.Kind())
	}
}
