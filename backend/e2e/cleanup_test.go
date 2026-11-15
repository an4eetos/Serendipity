//go:build e2e

package e2e

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

const cleanupTimeout = 2 * time.Minute

// cleanupStep is one idempotent part of the cleanup. It returns a note for
// the log.
type cleanupStep struct {
	name string
	run  func(ctx context.Context) (string, error)
}

func (s *scenario) cleanupSteps() []cleanupStep {
	return []cleanupStep{
		{"the server is stopped", s.stopServer},
		{"the storage objects are deleted", s.deleteObjects},
		{"the 3 users are deleted", s.deleteUsers},
		{"the works are deleted by id", s.deleteWorks},
		{"no tagged users works or objects are left", s.checkNothingTagged},
		{"the row counts match the baseline", s.checkBaseline},
	}
}

// stepCleanup runs the cleanup as checks. When every check passes, the
// final cleanup has nothing left to do.
func (s *scenario) stepCleanup(t *testing.T) {
	ok := true
	for _, st := range s.cleanupSteps() {
		ok = t.Run(st.name, func(t *testing.T) {
			note, err := st.run(t.Context())
			if note != "" {
				t.Log(note)
			}
			if err != nil {
				t.Fatal(err)
			}
		}) && ok
	}
	s.cleaned = ok
}

// finalCleanup runs when the test ends. Unless stepCleanup already
// succeeded (a step failed, or it never ran), it runs the cleanup again,
// reporting errors without stopping.
func (s *scenario) finalCleanup(t *testing.T) {
	if !s.cleaned {
		ctx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
		defer cancel()
		for _, st := range s.cleanupSteps() {
			note, err := st.run(ctx)
			if err != nil {
				t.Errorf("cleanup: %s: %v", st.name, err)
			} else if note != "" {
				t.Logf("cleanup: %s: %s", st.name, note)
			}
		}
	}
	if s.db != nil {
		s.db.Close()
	}
	if s.tmpDir != "" {
		if err := os.RemoveAll(s.tmpDir); err != nil {
			t.Errorf("remove %s: %v", s.tmpDir, err)
		}
	}
}

func (s *scenario) stopServer(context.Context) (string, error) {
	if s.srv == nil {
		return "no server was started", nil
	}
	if err := s.srv.stop(); err != nil {
		return "", err
	}
	if !s.srv.exited() {
		return "", errors.New("the server is still running")
	}
	return "", nil
}

// deleteObjects deletes every path the scenario may have written, and
// anything else in the users' folders, through the Storage API.
func (s *scenario) deleteObjects(ctx context.Context) (string, error) {
	if s.db == nil || len(s.users) == 0 {
		return "no users were created", nil
	}
	found, err := objectNames(ctx, s.db, s.users)
	if err != nil {
		return "", err
	}
	paths := dedupe(append(slices.Clone(s.objects), found...))
	var errs []error
	for _, p := range paths {
		errs = append(errs, s.sb.deleteObject(ctx, p))
	}
	return fmt.Sprintf("%d object(s) present before deleting", len(found)), errors.Join(errs...)
}

// deleteUsers deletes the users through the auth admin API, which cascades
// to their profiles, library, quotes, discussions, reports, takedowns and
// admins row. Users carrying the tag are included, in case one was created
// but its id was lost.
func (s *scenario) deleteUsers(ctx context.Context) (string, error) {
	if s.db == nil {
		return "no users were created", nil
	}
	tagged, err := taggedUserIDs(ctx, s.db, s.tag)
	if err != nil {
		return "", err
	}
	ids := dedupe(append(slices.Clone(s.users), tagged...))
	var errs []error
	for _, id := range ids {
		errs = append(errs, s.sb.deleteUser(ctx, id))
	}
	return fmt.Sprintf("%d user(s) present before deleting", len(tagged)), errors.Join(errs...)
}

// deleteWorks deletes the works the scenario created (they outlive their
// creator), by id, plus any work carrying the tag.
func (s *scenario) deleteWorks(ctx context.Context) (string, error) {
	if s.db == nil {
		return "no works were created", nil
	}
	tagged, err := taggedWorkIDs(ctx, s.db, s.tag)
	if err != nil {
		return "", err
	}
	ids := dedupe(append(slices.Clone(s.works), tagged...))
	if len(ids) == 0 {
		return "no works to delete", nil
	}
	n, err := deleteWorks(ctx, s.db, ids)
	return fmt.Sprintf("deleted %d work(s) by id", n), err
}

// checkNothingTagged looks for leftovers in auth, works and storage.
func (s *scenario) checkNothingTagged(ctx context.Context) (string, error) {
	if s.db == nil {
		return "no database connection", nil
	}
	users, err := taggedUserIDs(ctx, s.db, s.tag)
	if err != nil {
		return "", err
	}
	works, err := taggedWorkIDs(ctx, s.db, s.tag)
	if err != nil {
		return "", err
	}
	objects, err := objectNames(ctx, s.db, s.users)
	if err != nil {
		return "", err
	}
	if len(users)+len(works)+len(objects) > 0 {
		return "", fmt.Errorf("left behind: %d user(s), %d work(s), %d object(s): %v %v %v",
			len(users), len(works), len(objects), users, works, objects)
	}
	return fmt.Sprintf("0 users, 0 works and 0 objects tagged %s", s.tag), nil
}

// checkBaseline compares the row counts with those before the run. Another
// client writing to the local database at the same time also shows up here.
func (s *scenario) checkBaseline(ctx context.Context) (string, error) {
	if s.db == nil || s.baseline == nil {
		return "no baseline was recorded", nil
	}
	now, err := countRows(ctx, s.db)
	if err != nil {
		return "", err
	}
	if diff := now.diff(s.baseline); len(diff) > 0 {
		return "", errors.New("row counts differ from the baseline: " + strings.Join(diff, "; "))
	}
	return fmt.Sprintf("%d tables match the baseline", len(countedTables)), nil
}

func dedupe(ids []string) []string {
	slices.Sort(ids)
	return slices.Compact(ids)
}
