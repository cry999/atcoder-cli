package command

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/cry999/atcoder-cli/api"
)

func (c *Command) FetchSampleIO(ctx context.Context) error {
	client := api.NewClient(c.family)
	defer client.Shutdown()

	tasks, err := client.FetchTaskList(ctx)
	if err != nil {
		return err
	}
	for _, task := range tasks {
		if err := os.Mkdir(task.Index, 0755); err != nil && !os.IsExist(err) {
			slog.ErrorContext(ctx, "failed to create task directory", slog.String("dir", task.Index), slog.String("err", err.Error()))
			return err
		}
		err = client.FetchSampleIOs(ctx, task)
		if err != nil {
			slog.ErrorContext(ctx, "failed to fetch sample IOs", slog.String("err", err.Error()))
			return err
		}
		for i, io := range task.SampleIOs {
			if err := os.WriteFile(filepath.Join(task.Index, fmt.Sprintf("input-%02d.txt", i)), []byte(strings.Join(io.Input, "\n")+"\n"), 0644); err != nil {
				slog.ErrorContext(ctx, "failed to write sample input file", slog.String("file", filepath.Join(task.Index, fmt.Sprintf("input-%s.txt", task.Index))), slog.String("err", err.Error()))
			}
			if err := os.WriteFile(filepath.Join(task.Index, fmt.Sprintf("output-%02d.txt", i)), []byte(strings.Join(io.Output, "\n")+"\n"), 0644); err != nil {
				slog.ErrorContext(ctx, "failed to write sample output file", slog.String("file", filepath.Join(task.Index, fmt.Sprintf("output-%s.txt", task.Index))), slog.String("err", err.Error()))
			}
		}
	}

	return nil
}
