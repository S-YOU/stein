package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/b4b4r07/stein/lang"
	"github.com/b4b4r07/stein/lang/loader"
	"github.com/b4b4r07/stein/lint"
	"github.com/hashicorp/hcl2/hcl"
)

// ApplyCommand is one of the subcommands
type ApplyCommand struct {
	CLI
	Option ApplyOption

	policyFiles map[string]*hcl.File
}

// ApplyOption is the options for ApplyCommand
type ApplyOption struct {
	PolicyPath string
}

func (c *ApplyCommand) flagSet() *flag.FlagSet {
	flags := flag.NewFlagSet("apply", flag.ExitOnError)
	flags.StringVar(&c.Option.PolicyPath, "policy", ".", "path to the policy files or the directory where policy files are located")
	flags.VisitAll(func(f *flag.Flag) {
		if s := os.Getenv(strings.ToUpper(envEnvPrefix + f.Name)); s != "" {
			f.Value.Set(s)
		}
	})
	return flags
}

// Run run apply command
func (c *ApplyCommand) Run(args []string) int {
	flags := c.flagSet()
	if err := flags.Parse(args); err != nil {
		return c.exit(err)
	}
	args = flags.Args()

	if len(args) == 0 {
		return c.exit(errors.New("No config files given as arguments"))
	}

	// policy path can take a string separated by a comma like below
	// => foo/a,bar/b,buz/a/b/c
	paths := strings.Split(c.Option.PolicyPath, ",")

	policy, err := loader.Load(paths...)
	if err != nil {
		return c.exit(err)
	}
	c.policyFiles = policy.Files

	data, diags := lang.Decode(policy.Body)
	if diags.HasErrors() {
		return c.exit(diags)
	}
	policy.Data = data

	// settings about linter are below
	linter := lint.NewLinter(policy)

	files, err := lint.Args(args)
	if err != nil {
		return c.exit(err)
	}

	var results []lint.Result
	for _, file := range files {
		result, err := linter.Run(file)
		if err != nil {
			return c.exit(err)
		}
		results = append(results, result)
	}

	for _, result := range results {
		linter.Print(result)
	}
	linter.PrintSummary(results...)

	return c.exit(linter.Status(results...))
}

// Synopsis returns synopsis
func (c *ApplyCommand) Synopsis() string {
	return "Applies a policy to arbitrary config files."
}

// Help returns help message
func (c *ApplyCommand) Help() string {
	var b bytes.Buffer
	flags := c.flagSet()
	flags.SetOutput(&b)
	flags.PrintDefaults()
	return fmt.Sprintf(
		"Usage of %s:\n  %s\n\nOptions:\n%s", flags.Name(), c.Synopsis(), b.String(),
	)
}

// exit overwides CLI.exit
func (c *ApplyCommand) exit(msg interface{}) int {
	wr := hcl.NewDiagnosticTextWriter(
		c.Stderr,      // writer to send messages to
		c.policyFiles, // the parser's file cache, for source snippets
		100,           // wrapping width
		true,          // generate colored/highlighted output
	)
	switch m := msg.(type) {
	case error:
		switch diags := m.(type) {
		case hcl.Diagnostics:
			if len(diags) == 0 {
				return 1
			}
			wr.WriteDiagnostic(diags[0])
			return 1
		}
	case lint.Status:
		return int(m)
	}
	return c.CLI.exit(msg)
}
