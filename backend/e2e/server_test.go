//go:build e2e

package e2e

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const (
	healthTimeout = 60 * time.Second
	stopTimeout   = 15 * time.Second
	logTailLines  = 40
)

// server is the API binary running on a free 127.0.0.1 port.
type server struct {
	base    string
	cmd     *exec.Cmd
	logPath string
	done    chan struct{}
	waitErr error
}

// buildServer compiles ./cmd/server into dir and returns the binary's path.
func buildServer(backendDir, dir string) (string, error) {
	bin := filepath.Join(dir, "server")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/server")
	cmd.Dir = backendDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("go build ./cmd/server: %w\n%s", err, out)
	}
	return bin, nil
}

// freePort asks the kernel for an unused local port.
func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// startServer runs bin with env plus API_ADDR on a free port, and waits for
// /healthz. Its output goes to dir/server.log.
func startServer(bin, dir string, env []string) (*server, error) {
	port, err := freePort()
	if err != nil {
		return nil, err
	}
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	s := &server{base: "http://" + addr, logPath: filepath.Join(dir, "server.log"), done: make(chan struct{})}
	logFile, err := os.Create(s.logPath)
	if err != nil {
		return nil, err
	}
	s.cmd = exec.Command(bin)
	s.cmd.Env = append(slices.Clone(env), "API_ADDR="+addr)
	s.cmd.Stdout = logFile
	s.cmd.Stderr = logFile
	if err := s.cmd.Start(); err != nil {
		logFile.Close()
		return nil, err
	}
	go func() {
		s.waitErr = s.cmd.Wait()
		logFile.Close()
		close(s.done)
	}()
	return s, s.waitHealthy()
}

// waitHealthy polls /healthz until it answers 200, the process exits or the
// timeout passes.
func (s *server) waitHealthy() error {
	hc := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(healthTimeout)
	for time.Now().Before(deadline) {
		select {
		case <-s.done:
			return fmt.Errorf("server exited during startup (%v); last log lines:\n%s", s.waitErr, s.logTail())
		default:
		}
		resp, err := hc.Get(s.base + "/healthz")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("server not healthy after %s; last log lines:\n%s", healthTimeout, s.logTail())
}

// stop interrupts the server, kills it if it doesn't exit in time, and
// waits for it.
func (s *server) stop() error {
	select {
	case <-s.done:
		return nil
	default:
	}
	if err := s.cmd.Process.Signal(os.Interrupt); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	select {
	case <-s.done:
	case <-time.After(stopTimeout):
		if err := s.cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			return err
		}
		<-s.done
	}
	return nil
}

// exited reports whether the process has finished.
func (s *server) exited() bool {
	select {
	case <-s.done:
		return true
	default:
		return false
	}
}

// logTail returns the last lines the server wrote.
func (s *server) logTail() string {
	raw, err := os.ReadFile(s.logPath)
	if err != nil {
		return "(no log: " + err.Error() + ")"
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if len(lines) > logTailLines {
		lines = lines[len(lines)-logTailLines:]
	}
	return strings.Join(lines, "\n")
}
