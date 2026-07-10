package sync

import (
	"errors"
	"fmt"
	"testing"
)

func TestIsTransient(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{errors.New("create gitlab project: 400 {message: {base: [Request timed out. Please try again.]}}"), true},
		{errors.New("dial tcp: i/o timeout"), true},
		{errors.New("connection reset by peer"), true},
		{errors.New("HTTP 503 Service Unavailable"), true},
		{errors.New("429 Too Many Requests"), true},
		{fmt.Errorf("mirror push: %w", errors.New("EOF")), true},
		{errors.New("authorization failed: You are not allowed to push"), false},
		{errors.New("command error on refs/heads/main: pre-receive hook declined"), false},
		{errors.New("401 Bad credentials"), false},
		{errors.New("403 Forbidden"), false},
		{errors.New("repository already exists"), false},
	}
	for _, tc := range cases {
		got := isTransient(tc.err)
		if got != tc.want {
			t.Fatalf("isTransient(%q)=%v want %v", tc.err, got, tc.want)
		}
	}
}
