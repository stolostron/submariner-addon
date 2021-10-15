package main

import (
	goflag "flag"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/open-cluster-management/submariner-addon/pkg/cmd/hub"
	"github.com/open-cluster-management/submariner-addon/pkg/cmd/spoke"
	"github.com/open-cluster-management/submariner-addon/pkg/version"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	utilflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"
)

// The submariner binary is used to integrate between ACM and Submariner.

func main() {
	rand.Seed(time.Now().UTC().UnixNano())

	pflag.CommandLine.SetNormalizeFunc(utilflag.WordSepNormalizeFunc)
	pflag.CommandLine.AddGoFlagSet(goflag.CommandLine)

	logs.InitLogs()

	exitCode := 0
	command := newSubmarinerControllerCommand()
	if err := command.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)

		exitCode = 1
	}

	logs.FlushLogs()

	os.Exit(exitCode)
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

	if v := version.Get().String(); v == "" {
		cmd.Version = "<unknown>"
	} else {
		cmd.Version = v
	}

	cmd.AddCommand(hub.NewController())
	cmd.AddCommand(spoke.NewAgent())

	return cmd
}
