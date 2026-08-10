package cmd

import (
	"fmt"
	"os"
)

const banner = "'||'''|, '||'''|, '||`                                \n" +
	" ||   ||  ||   ||  ||                                 \n" +
	" ||...|'  ||...|'  ||  .|''|, .|''|, '||  ||` .|''|, \n" +
	" ||       || \\\\    ||  ||  || ||  ||  ||  ||  ||..|| \n" +
	".||      .||  \\\\. .||. `|..|' `|..||  `|..'|. `|...  \n" +
	"                                  ||                 \n" +
	"                               `..|' "

// printBanner prints a blank line and the PRlogue banner at the start of every
// CLI invocation. The blue color is only emitted when stderr is a terminal, so
// piped or redirected output stays free of escape codes.
func printBanner() {
	if isTerminal(os.Stderr) {
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "\033[34m"+banner+"\033[0m")
		return
	}
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, banner)
}

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
