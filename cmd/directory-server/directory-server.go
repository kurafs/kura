package directoryserver

import (
	"fmt"

	"github.com/kurafs/kura/pkg/cli"
)

var DirectoryServerCmd = &cli.Command{
	Run:       directoryServerCmdRun,
	UsageLine: "directory-server [-f] [-a arg]",
	Short:     "directory-server command overview",
	Long: `
Directory server detailed overview.
    `,
}

func directoryServerCmdRun(cmd *cli.Command, args []string) error {
	f := cmd.FlagSet.Bool("f", false, "Flag usage")
	a := cmd.FlagSet.String("a", "", "Argument parameter usage")
	if err := cmd.FlagSet.Parse(args); err != nil {
		return cli.CmdParseError(err)
	}

	fmt.Println(fmt.Sprintf("%s: parsed successfully: %t, %s", cmd.Name(), *f, *a))
	return nil
}
