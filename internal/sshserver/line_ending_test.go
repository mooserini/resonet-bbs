package sshserver

import (
	"bufio"
	"io"
	"testing"
	"time"
)

// A bare CR must end the line right away. The old code peeked for a trailing
// LF, which blocked until the next keystroke, so the password prompt only
// appeared after another key and a second Enter bounced back to Handle.
func TestReadLineBareCRDoesNotWaitForNextKey(t *testing.T) {
	pr, pw := io.Pipe()
	reader := bufio.NewReader(pr)
	done := make(chan string, 1)
	go func() {
		line, _ := readLine(reader, 32)
		done <- line
	}()
	_, _ = pw.Write([]byte("Moose\r"))
	select {
	case line := <-done:
		if line != "Moose" {
			t.Fatalf("got %q", line)
		}
	case <-time.After(time.Second):
		t.Fatal("readLine blocked waiting for a byte after CR")
	}

	// A late LF (or NUL) belonging to that Enter is swallowed, not read as an empty line.
	go func() {
		line, _ := readLine(reader, 32)
		done <- line
	}()
	_, _ = pw.Write([]byte("\n"))
	_, _ = pw.Write([]byte("secret\r"))
	select {
	case line := <-done:
		if line != "secret" {
			t.Fatalf("late LF produced %q, want secret", line)
		}
	case <-time.After(time.Second):
		t.Fatal("second readLine did not return")
	}
	_ = pw.Close()
}

func TestReadLineEmptyEnterStillWorks(t *testing.T) {
	pr, pw := io.Pipe()
	reader := bufio.NewReader(pr)
	done := make(chan string, 2)
	go func() {
		for i := 0; i < 2; i++ {
			line, _ := readLine(reader, 32)
			done <- line
		}
	}()
	_, _ = pw.Write([]byte("\r"))
	_, _ = pw.Write([]byte("\r"))
	for i := 0; i < 2; i++ {
		select {
		case line := <-done:
			if line != "" {
				t.Fatalf("got %q, want empty line", line)
			}
		case <-time.After(time.Second):
			t.Fatal("two Enters should give two empty lines")
		}
	}
	_ = pw.Close()
}
