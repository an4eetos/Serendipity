package postgres

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestIsCheckViolationOf(t *testing.T) {
	named := &pgconn.PgError{Code: codeCheckViolation, ConstraintName: "reports_not_own_content"}
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"the named check", named, true},
		{"the named check, wrapped", fmt.Errorf("insert: %w", named), true},
		{"another check", &pgconn.PgError{Code: codeCheckViolation, ConstraintName: "reports_note_check"}, false},
		{"an unnamed check", &pgconn.PgError{Code: codeCheckViolation}, false},
		{"another code with the name", &pgconn.PgError{Code: codeUniqueViolation, ConstraintName: "reports_not_own_content"}, false},
		{"not a Postgres error", errors.New("reports_not_own_content"), false},
		{"nil", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsCheckViolationOf(tt.err, "reports_not_own_content"); got != tt.want {
				t.Errorf("IsCheckViolationOf() = %v, want %v", got, tt.want)
			}
		})
	}
}
