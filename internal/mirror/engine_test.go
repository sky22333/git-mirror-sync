package mirror

import (
	"testing"
)

func TestBackupRefSpecs(t *testing.T) {
	want := []string{
		"+refs/heads/*:refs/heads/*",
		"+refs/tags/*:refs/tags/*",
	}
	if len(backupRefSpecs) != len(want) {
		t.Fatalf("len=%d want %d", len(backupRefSpecs), len(want))
	}
	for i := range want {
		if string(backupRefSpecs[i]) != want[i] {
			t.Fatalf("refspec[%d]=%q want %q", i, backupRefSpecs[i], want[i])
		}
	}
}
