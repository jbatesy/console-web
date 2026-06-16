package pty_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"console-web/internal/pty"
)

func TestScrollbackTrim(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pane.log")

	// Write 20 bytes, max is 10 — should trim to last 10 bytes (keep maxBytes/2 = 5... wait)
	// TrimScrollback keeps the last maxBytes/2 bytes when file > maxBytes
	// file size=20, maxBytes=10, keep=maxBytes/2=5, offset=20-5=15
	// last 5 bytes of "0123456789abcdefghij" = "fghij"
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	f.Write([]byte("0123456789abcdefghij")) // 20 bytes
	f.Close()

	if err := pty.TrimScrollback(path, 10); err != nil {
		t.Fatalf("trim: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// maxBytes=10, keep=maxBytes/2=5, last 5 bytes = "fghij"
	if string(data) != "fghij" {
		t.Errorf("after trim: %q (len %d), want %q", data, len(data), "fghij")
	}
}

func TestScrollbackTrimBelowMax(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pane.log")
	os.WriteFile(path, []byte("hello"), 0644)

	if err := pty.TrimScrollback(path, 100); err != nil {
		t.Fatalf("trim: %v", err)
	}

	data, _ := os.ReadFile(path)
	if string(data) != "hello" {
		t.Errorf("should be unchanged, got %q", data)
	}
}

func TestSpawnAndOutput(t *testing.T) {
	dir := t.TempDir()
	m := pty.NewManager(dir, 1024*1024)

	outPath := filepath.Join(dir, "out.log")
	paneID, err := m.Spawn("pane-test", "echo hello-from-pty", outPath)
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	if paneID != "pane-test" {
		t.Errorf("paneID: %q", paneID)
	}

	// Give the process time to write output and exit
	time.Sleep(300 * time.Millisecond)

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if !strings.Contains(string(data), "hello-from-pty") {
		t.Errorf("output file missing expected string, got: %q", data)
	}
}
