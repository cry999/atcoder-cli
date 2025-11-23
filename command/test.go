package command

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/google/go-cmp/cmp"
)

func (c *Command) RunTest(ctx context.Context, taskIndex string) error {
	// TODO: language

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
		fmt.Println(samples)
		return nil
	})
	for number, sample := range samples {
		fmt.Printf("Test case %s:\n", number)
		// TODO: ファイル名の共通化

		inputFile, err := os.Open(sample.input)
		if err != nil {
			return err
		}
		defer inputFile.Close()

		var output, errout bytes.Buffer

		python3 := exec.CommandContext(ctx, "python3", fmt.Sprintf("%s/main.py", taskIndex))
		python3.Stdin = inputFile
		python3.Stdout = &output
		python3.Stderr = &errout

		if err := python3.Run(); err != nil {
			return fmt.Errorf("%s: %w", errout.String(), err)
		}

		expect, err := os.ReadFile(sample.output)
		if err != nil {
			return err
		}
		fmt.Printf("Output:\n%s\n", output.String())
		diff := cmp.Diff(
			strings.Split(string(expect), "\n"),
			strings.Split(output.String(), "\n"),
		)
		if diff == "" {
			fmt.Println("AC")
		} else {
			fmt.Println("WA")
			fmt.Printf("Diff:\n%s\n", diff)
		}
	}
	return nil
}
