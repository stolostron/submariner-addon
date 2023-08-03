package hub

import (
	"context"

	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"github.com/spf13/cobra"
	base "github.com/stolostron/submariner-addon/pkg/cmd"
	"github.com/stolostron/submariner-addon/pkg/hub"
	"github.com/stolostron/submariner-addon/pkg/version"
)

func NewController() *cobra.Command {
	addOnOptions := hub.NewAddOnOptions()
	cmdConfig := controllercmd.
		NewControllerCommandConfig("submariner-controller", version.Get(), addOnOptions.RunControllerManager)
	cmd := cmdConfig.NewCommandWithContext(context.Background())
	cmd.Use = "controller"
	cmd.Short = "Start the ACM Submariner Controller"

	base.AddLeaderElectionFlags(cmd.Flags(), cmdConfig)
	addOnOptions.AddFlags(cmd)

	return cmd
}
