package cmd

import "testing"

func TestQuietFlag(t *testing.T) {
	flag := rootCmd.PersistentFlags().Lookup("quiet")
	if flag == nil {
		t.Fatal("quiet flag is not registered")
	}
	if flag.Shorthand != "q" {
		t.Errorf("quiet shorthand = %q, want q", flag.Shorthand)
	}
}
