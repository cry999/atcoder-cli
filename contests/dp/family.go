package dp

import (
	"path/filepath"

	"github.com/cry999/atcoder-cli/contests"
)

// New creates a new DP contest family.
func New() (contests.Family, error) {
	return &family{}, nil
}

type family struct{}

func (f *family) ContestName() string {
	return "dp"
}

func (f *family) BaseDir(workdir string) string {
	return filepath.Join(workdir, "dp")
}
