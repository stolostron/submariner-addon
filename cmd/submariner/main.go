package main

import (
	goflag "flag"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/stolostron/submariner-addon/pkg/cmd/hub"
	"github.com/stolostron/submariner-addon/pkg/cmd/spoke"

	utilflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"

	"github.com/stolostron/submariner-addon/pkg/version"
)

// The submariner binary is used to integrate between ACM and Submariner.

func main() {
	rand.Seed(time.Now().UTC().UnixNano())

	pflag.CommandLine.SetNormalizeFunc(utilflag.WordSepNormalizeFunc)
	pflag.CommandLine.AddGoFlagSet(goflag.CommandLine)

	logs.InitLogs()
	defer logs.FlushLogs()

	command := newSubmarinerControllerCommand()
	if err := command.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func newSubmarinerControllerCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "submariner",
		Short: "submariner-addon",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
			os.Exit(1)
		},
	}

	if v := version.Get().String(); len(v) == 0 {
		cmd.Version = "<unknown>"
	} else {
		cmd.Version = v
	}

	cmd.AddCommand(hub.NewController())
	cmd.AddCommand(spoke.NewAgent())

	return cmd
}
