package command

import (
	"context"
	"log/slog"
	"os"

	"github.com/cry999/atcoder-cli/contests"
)

type Command struct {
	family contests.Family
}

func NewCommand(ctx context.Context, family contests.Family, workdir string) (*Command, error) {
	baseDir := family.BaseDir(workdir)
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		slog.ErrorContext(ctx, "failed to create working directory", slog.String("dir", baseDir), slog.String("err", err.Error()))
		return nil, err
	}
	if err := os.Chdir(baseDir); err != nil {
		slog.ErrorContext(ctx, "failed to change working directory", slog.String("dir", baseDir), slog.String("err", err.Error()))
		return nil, err
	}

	return &Command{
		family: family,
	}, nil
}
