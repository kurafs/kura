package identityserver

import (
	"fmt"

	"github.com/kurafs/kura/pkg/cli"
)

var IdentityServerCmd = &cli.Command{
	Run:       identityServerCmdRun,
	UsageLine: "identity-server [-f] [-a arg]",
	Short:     "identity-server command overview",
	Long: `
Identity server detailed overview.
    `,
}

func identityServerCmdRun(cmd *cli.Command, args []string) error {
	f := cmd.FlagSet.Bool("f", false, "Flag usage")
	a := cmd.FlagSet.String("a", "", "Argument parameter usage")
	if err := cmd.FlagSet.Parse(args); err != nil {
		return cli.CmdParseError(err)
	}

	fmt.Println(fmt.Sprintf("%s: parsed successfully: %t, %s", cmd.Name(), *f, *a))
	return nil
}
