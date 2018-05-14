package storageserver

import (
	"fmt"

	"github.com/kurafs/kura/pkg/cli"
)

var StorageServerCmd = &cli.Command{
	Run:       storageServerCmdRun,
	UsageLine: "storage-server [-f] [-a arg]",
	Short:     "storage-server command overview",
	Long: `
Storage server detailed overview.
    `,
}

func storageServerCmdRun(cmd *cli.Command, args []string) error {
	f := cmd.FlagSet.Bool("f", false, "Flag usage")
	a := cmd.FlagSet.String("a", "", "Argument parameter usage")
	if err := cmd.FlagSet.Parse(args); err != nil {
		return cli.CmdParseError(err)
	}

	fmt.Println(fmt.Sprintf("%s: parsed successfully: %t, %s", cmd.Name(), *f, *a))
	return nil
}
