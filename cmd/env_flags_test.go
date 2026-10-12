package cmd

import (
	"strings"
	"testing"

	"github.com/samber/lo"
	"github.com/spf13/pflag"
)

// Every flag can be set with the environment variable FB_<FLAG IN UPPER_SNAKE
// CASE>. The names are built with a replacer that goes through the flags in
// order, so a flag that is the prefix of another one (--webdav and
// --webdavPassword) makes the longer flag's variable stop working; this test
// catches that for all flags, present and future.
func TestEveryFlagCanBeSetThroughItsEnvironmentVariable(t *testing.T) {
	var flags []*pflag.Flag
	rootCmd.Flags().VisitAll(func(f *pflag.Flag) {
		if f.Name != "config" && f.Name != "help" && f.Name != "version" {
			flags = append(flags, f)
		}
	})
	if len(flags) < 30 {
		t.Fatalf("only %d flags found", len(flags))
	}

	for _, f := range flags {
		name := "FB_" + strings.ToUpper(lo.SnakeCase(f.Name))
		t.Setenv(name, "value-of-"+f.Name)
	}

	// initViper reads --config from the command's flags.
	if err := rootCmd.ParseFlags(nil); err != nil {
		t.Fatal(err)
	}
	v, err := initViper(rootCmd)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range flags {
		want := "value-of-" + f.Name
		if got := v.GetString(f.Name); got != want {
			t.Errorf("flag --%s: the environment variable FB_%s gave %q, want %q",
				f.Name, strings.ToUpper(lo.SnakeCase(f.Name)), got, want)
		}
	}
}
