package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"

	"github.com/cry999/atcoder-cli/command"
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
		testcase   = flag.String("testcase", "all", "Which testcases to run (all, 0, 1, 2, ...)")
		verbose    = flag.Bool("v", false, "Enable verbose logging")
	)
	flag.Parse()

	if *dumpConfig {
		if err := config.Dump(os.Stdout); err != nil {
			slog.ErrorContext(ctx, "failed to dump config", slog.String("err", err.Error()))
		}
		return
	}

	contestFamily := flag.Arg(1)
	if contestFamily == "" {
		slog.ErrorContext(ctx, "contest family argument is required")
		return
	}

	var family contests.Family
	switch contestFamily {
	// AtCoder Daily Training
	case "adt":
		family, err = adt.New(flag.Arg(2), flag.Arg(3), *adtLevel)
		if err != nil {
			slog.ErrorContext(
				ctx, "failed to parse ADT family",
				slog.String("date", flag.Arg(2)),
				slog.String("time", flag.Arg(3)),
				slog.String("err", err.Error()),
			)
			return
		}
	default:
		slog.ErrorContext(ctx, "unknown contest type", slog.String("contest", contestFamily))
		return
	}

	cmd, err := command.NewCommand(ctx, family, config.WorkDir)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create command", slog.String("err", err.Error()))
		return
	}

	switch flag.Arg(0) {
	case "init":
		if err := cmd.FetchSampleIO(ctx); err != nil {
			slog.ErrorContext(ctx, "failed to fetch sample IO", slog.String("err", err.Error()))
			return
		}
	case "test":
		taskIndex := flag.Arg(4)
		if taskIndex == "" {
			fmt.Println("task index argument is required for test command")
			return
		}
		var opts command.TestOptions
		if *testcase != "all" {
			opts = append(opts, command.TestWithTestcase(*testcase))
		}
		if *verbose {
			opts = append(opts, command.TestWithVerbose())
		}
		if err := cmd.RunTest(ctx, taskIndex, opts...); err != nil {
			slog.ErrorContext(ctx, "failed to run tests", slog.String("err", err.Error()))
			return
		}
	default:
		slog.ErrorContext(ctx, "unknown command", slog.String("command", flag.Arg(0)))
		return
	}
}
