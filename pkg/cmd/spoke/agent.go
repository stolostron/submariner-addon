package spoke

import (
	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"github.com/spf13/cobra"
	"github.com/stolostron/submariner-addon/pkg/spoke"
	"github.com/stolostron/submariner-addon/pkg/version"
	"k8s.io/utils/clock"
)

func NewAgent() *cobra.Command {
	agentOptions := spoke.NewAgentOptions()
	cmd := controllercmd.
		NewControllerCommandConfig("submariner-agent", version.Get(), agentOptions.RunAgent, clock.RealClock{}).
		NewCommand()
	cmd.Use = "agent"
	cmd.Short = "Start the ACM Submariner Agent"

	agentOptions.AddFlags(cmd)

	return cmd
}
