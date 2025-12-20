package command

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/google/go-cmp/cmp"
)

var styleAC = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("#FAFAFA")).
	Background(lipgloss.Color("#43A047")).
	PaddingLeft(1).PaddingRight(1)

var styleWA = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("#FAFAFA")).
	Background(lipgloss.Color("#E53935")).
	PaddingLeft(1).PaddingRight(1)

var styleDiffPlus = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("#43A047")).
	PaddingLeft(1).PaddingRight(1)

var styleDiffMinus = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("#E53935")).
	PaddingLeft(1).PaddingRight(1)

var styleTitle = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("#1E88E5")).
	PaddingLeft(1).PaddingRight(1)

var styleSkip = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("#FAFAFA")).
	Background(lipgloss.Color("#757575")).
	PaddingLeft(1).PaddingRight(1)

type TestOptions []TestOption

type TestOption func(*testConfig)

type testConfig struct {
	testcase string
	verbose  bool
}

func TestWithTestcase(testcase string) TestOption {
	return func(tc *testConfig) {
		tc.testcase = testcase
		// tc.verbose = testcase != "all"
	}
}

func TestWithVerbose() TestOption {
	return func(tc *testConfig) {
		tc.verbose = true
	}
}

func (c *Command) RunTest(ctx context.Context, taskIndex string, opts ...TestOption) error {
	// TODO: language

	cfg := testConfig{
		testcase: "all",
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	type sample struct {
		input, output string
	}

	samples := map[string]sample{}

	fs.WalkDir(os.DirFS("."), taskIndex, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		filename := filepath.Base(path)
		if after, ok := strings.CutPrefix(filename, "input-"); ok {
			number := strings.TrimSuffix(after, ".txt")
			samples[number] = sample{input: path, output: samples[number].output}
		} else if after, ok := strings.CutPrefix(filename, "output-"); ok {
			number := strings.TrimSuffix(after, ".txt")
			samples[number] = sample{input: samples[number].input, output: path}
		} else {
			return nil
		}
		return nil
	})
	for number, sample := range samples {
		if cfg.testcase != "all" && number != cfg.testcase {
			if cfg.verbose {
				fmt.Printf("%s test case %s\n", styleSkip.Render("SKIP"), number)
			}
			continue
		}
		// TODO: ファイル名の共通化

		inputFile, err := os.Open(sample.input)
		if err != nil {
			return err
		}
		defer inputFile.Close()

		execfile := fmt.Sprintf("%s/main.py", taskIndex)
		if _, err := os.Stat(execfile); err != nil && os.IsNotExist(err) {
			fmt.Println(styleWA.Render("Error:"))
			fmt.Println("No such file:", execfile)
		}

		var input, output, errout bytes.Buffer
		python3 := exec.CommandContext(ctx, "python3", execfile)
		python3.Stdin = io.TeeReader(inputFile, &input)
		python3.Stdout = &output
		python3.Stderr = &errout

		if err := python3.Run(); err != nil {
			fmt.Printf("%s: Test case %s:\n", styleWA.Render("ERROR"), number)
			fmt.Println(styleTitle.Render("Input:"))
			fmt.Println(input.String())
			fmt.Println(styleTitle.Render("Output:"))
			fmt.Println(output.String())
			fmt.Println(styleWA.Render("Error:"))
			fmt.Println(errout.String())
			return fmt.Errorf("%s: %w", errout.String(), err)
		}

		expect, err := os.ReadFile(sample.output)
		if err != nil {
			return err
		}
		diff := cmp.Diff(
			strings.Split(string(expect), "\n"),
			strings.Split(output.String(), "\n"),
		)

		var result string
		if diff == "" {
			result = styleAC.Render("AC")
		} else {
			result = styleWA.Render("WA")
		}
		fmt.Printf("%s Test case %s\n", result, number)
		if diff != "" || cfg.verbose {
			fmt.Printf("%s:\n%s\n", styleTitle.Render("Input"), input.String())
			fmt.Printf("%s:\n%s\n", styleTitle.Render("Debug"), errout.String())
			fmt.Printf("%s:\n%s\n", styleTitle.Render("Output"), output.String())
			fmt.Printf("%s: %s\n", styleTitle.Render("Result"), result)
			fmt.Println("Diff:")
			for line := range strings.SplitSeq(diff, "\n") {
				switch {
				case strings.HasPrefix(line, "+"):
					fmt.Println(styleDiffPlus.Render(line))
				case strings.HasPrefix(line, "-"):
					fmt.Println(styleDiffMinus.Render(line))
				default:
					fmt.Println(line)
				}
			}
		}
		fmt.Println()
	}
	return nil
}
