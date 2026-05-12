package hub

import (
	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"github.com/spf13/cobra"
	"github.com/stolostron/submariner-addon/pkg/hub"
	"github.com/stolostron/submariner-addon/pkg/version"
	"k8s.io/utils/clock"
)

func NewController() *cobra.Command {
	addOnOptions := hub.NewAddOnOptions()

	// Create base command config with serving disabled
	cmdConfig := controllercmd.
		NewControllerCommandConfig("submariner-controller", version.Get(), addOnOptions.RunControllerManager, clock.RealClock{})

	// Disable controllercmd's HTTPS server - we'll use our own HTTP server
	cmdConfig.DisableServing = true

	cmd := cmdConfig.NewCommand()
	cmd.Use = "controller"
	cmd.Short = "Start the ACM Submariner Controller"

	addOnOptions.AddFlags(cmd)

	return cmd
}
