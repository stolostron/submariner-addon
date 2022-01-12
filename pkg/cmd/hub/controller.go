package hub

import (
	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"github.com/spf13/cobra"
	"github.com/stolostron/submariner-addon/pkg/hub"
	"github.com/stolostron/submariner-addon/pkg/version"
)

func NewController() *cobra.Command {
	addOnOptions := hub.NewAddOnOptions()
	cmd := controllercmd.
		NewControllerCommandConfig("submariner-controller", version.Get(), addOnOptions.RunControllerManager).
		NewCommand()
	cmd.Use = "controller"
	cmd.Short = "Start the ACM Submariner Controller"

	addOnOptions.AddFlags(cmd)

	return cmd
}
