package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/cry999/atcoder-cli/api"
	"github.com/cry999/atcoder-cli/config"
	"github.com/cry999/atcoder-cli/contests"
	"github.com/cry999/atcoder-cli/contests/adt"
)

func init() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	config, err := config.LoadConfig(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to load config", slog.String("err", err.Error()))
		return
	}

	var (
		dumpConfig = flag.Bool("dump-config", false, "Dump loaded config and exit")
		adtLevel   = flag.String("adt-level", string(config.ADT.DefaultLevel), "Default level for ADT problems (easy, medium, hard, all)")
	)
	flag.Parse()

	if *dumpConfig {
		if err := config.Dump(os.Stdout); err != nil {
			slog.ErrorContext(ctx, "failed to dump config", slog.String("err", err.Error()))
		}
		return
	}

	contestFamily := flag.Arg(0)
	if contestFamily == "" {
		slog.ErrorContext(ctx, "contest family argument is required")
		return
	}

	var family contests.Family
	switch contestFamily {
	// AtCoder Daily Training
	case "adt":
		family, err = adt.New(flag.Arg(1), flag.Arg(2), *adtLevel)
		if err != nil {
			slog.ErrorContext(
				ctx, "failed to parse ADT family",
				slog.String("date", flag.Arg(1)),
				slog.String("time", flag.Arg(2)),
				slog.String("err", err.Error()),
			)
			return
		}
	default:
		slog.ErrorContext(ctx, "unknown contest type", slog.String("contest", contestFamily))
		return
	}

	baseDir := family.BaseDir(config.WorkDir)
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		slog.ErrorContext(ctx, "failed to create working directory", slog.String("dir", baseDir), slog.String("err", err.Error()))
		return
	}
	if err := os.Chdir(baseDir); err != nil {
		slog.ErrorContext(ctx, "failed to change working directory", slog.String("dir", baseDir), slog.String("err", err.Error()))
		return
	}

	client := api.NewClient(family)

	tasks, err := client.FetchTaskList(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to fetch task list", slog.String("err", err.Error()))
		return
	}
	for _, task := range tasks {
		if err := os.Mkdir(task.Index, 0755); err != nil && !os.IsExist(err) {
			slog.ErrorContext(ctx, "failed to create task directory", slog.String("dir", task.Index), slog.String("err", err.Error()))
			return
		}
		err = client.FetchSampleIOs(ctx, task)
		if err != nil {
			slog.ErrorContext(ctx, "failed to fetch sample IOs", slog.String("err", err.Error()))
			return
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
}
