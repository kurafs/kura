package blobserver

import (
	"fmt"

	"github.com/kurafs/kura/pkg/cli"
)

var BlobServerCmd = &cli.Command{
	Run:       blobServerCmdRun,
	UsageLine: "blob-server [-f] [-a arg]",
	Short:     "blob-server command overview",
	Long: `
Blob server detailed overview.
    `,
}

func blobServerCmdRun(cmd *cli.Command, args []string) error {
	f := cmd.FlagSet.Bool("f", false, "Flag usage")
	a := cmd.FlagSet.String("a", "", "Argument parameter usage")
	if err := cmd.FlagSet.Parse(args); err != nil {
		return cli.CmdParseError(err)
	}

	fmt.Println(fmt.Sprintf("%s: parsed successfully: %t, %s", cmd.Name(), *f, *a))
	return nil
}
