package postgres

import (
	"context"
	"testing"

	kernelpg "github.com/an4eetos/serendipity/backend/internal/kernel/adapters/postgres"
	"github.com/an4eetos/serendipity/backend/internal/kernel/adapters/postgres/pgtest"
)

func TestProfileVisibility(t *testing.T) {
	pgtest.Run(t, func(ctx context.Context, db *kernelpg.DB) {
		public, private := pgtest.NewUser(t, ctx, db), pgtest.NewUser(t, ctx, db)
		if _, err := db.Exec(ctx, `update profiles set is_public = true where id = $1`, public); err != nil {
			t.Fatal(err)
		}
		repo := NewProfileVisibility(db)
		for _, tt := range []struct {
			name, userID string
			want         bool
		}{
			{"public profile", public, true},
			{"private profile", private, false},
			{"no such profile", "00000000-0000-0000-0000-000000000000", false},
		} {
			if got, err := repo.IsPublic(ctx, tt.userID); err != nil || got != tt.want {
				t.Errorf("%s: IsPublic() = %v, %v; want %v", tt.name, got, err, tt.want)
			}
		}
	})
}
