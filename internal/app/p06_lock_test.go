package app

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"

	"github.com/desanv01/cuemender/internal/datalock"
)

func p06Port(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	return addr
}

func TestP06SameDataDirectoryRejectedAcrossPorts(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	a, err := New(ctx, Options{DataDir: dir, ListenAddress: p06Port(t), NoBrowser: true, SessionOnlySecrets: true, Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatal(err)
	}
	b, err := New(ctx, Options{DataDir: dir, ListenAddress: p06Port(t), NoBrowser: true, SessionOnlySecrets: true, Stdout: io.Discard, Stderr: io.Discard})
	if b != nil {
		b.Close()
	}
	if !errors.Is(err, datalock.ErrLocked) {
		t.Fatalf("second instance error=%v", err)
	}
	a.Close()
	c, err := New(ctx, Options{DataDir: dir, ListenAddress: p06Port(t), NoBrowser: true, SessionOnlySecrets: true, Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("lock not released after close: %v", err)
	}
	c.Close()
	other, err := New(ctx, Options{DataDir: t.TempDir(), ListenAddress: p06Port(t), NoBrowser: true, SessionOnlySecrets: true, Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("separate directory rejected: %v", err)
	}
	other.Close()
}

func TestP06FailedStartupReleasesLock(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	blocker, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := blocker.Addr().String()
	defer blocker.Close()
	if a, err := New(ctx, Options{DataDir: dir, ListenAddress: addr, NoBrowser: true, SessionOnlySecrets: true, Stdout: io.Discard, Stderr: io.Discard}); err == nil {
		a.Close()
		t.Fatal("startup unexpectedly succeeded on occupied port")
	}
	a, err := New(ctx, Options{DataDir: dir, ListenAddress: p06Port(t), NoBrowser: true, SessionOnlySecrets: true, Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("failed startup retained data lock: %v", err)
	}
	a.Close()
}
