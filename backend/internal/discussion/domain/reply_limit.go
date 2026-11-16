package domain

import (
	"time"

	kernel "github.com/an4eetos/serendipity/backend/internal/kernel/domain"
)

// ReplyWindow is the period ReplyLimit counts over.
const ReplyWindow = time.Hour

// ErrReplyLimit is returned when a reader has asked for too many character
// replies within ReplyWindow.
var ErrReplyLimit = kernel.RateLimited("you've reached the hourly limit of character replies — try again later")

// ReplyLimit is how many character replies a reader may ask for within
// ReplyWindow. Every reply is a paid model call, so the limit caps what one
// reader can spend.
type ReplyLimit int

// CheckRoom reports whether a reader who asked for recent replies within
// ReplyWindow may ask for one more.
func (l ReplyLimit) CheckRoom(recent int) error {
	if recent >= int(l) {
		return ErrReplyLimit
	}
	return nil
}
