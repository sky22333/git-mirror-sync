package model_test

import (
	"testing"

	"git-mirror-sync/internal/model"
)

func TestResolvePrivate(t *testing.T) {
	cases := []struct {
		policy model.VisibilityPolicy
		source bool
		want   bool
	}{
		{model.VisibilityPrivate, false, true},
		{model.VisibilityPrivate, true, true},
		{model.VisibilityPublic, true, false},
		{model.VisibilityPublic, false, false},
		{model.VisibilityFollow, true, true},
		{model.VisibilityFollow, false, false},
	}
	for _, tc := range cases {
		got := tc.policy.ResolvePrivate(tc.source)
		if got != tc.want {
			t.Fatalf("%s sourcePrivate=%v: got %v want %v", tc.policy, tc.source, got, tc.want)
		}
	}
}
