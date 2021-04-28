package spoke

import (
	"github.com/spf13/cobra"

	"github.com/openshift/library-go/pkg/controller/controllercmd"

	"github.com/open-cluster-management/submariner-addon/pkg/spoke"
	"github.com/open-cluster-management/submariner-addon/pkg/version"
)

func NewAgent() *cobra.Command {
	agentOptions := spoke.NewAgentOptions()
	cmd := controllercmd.
		NewControllerCommandConfig("submariner-agent", version.Get(), agentOptions.RunAgent).
		NewCommand()
	cmd.Use = "agent"
	cmd.Short = "Start the ACM Submariner Agent"

	agentOptions.AddFlags(cmd)
	return cmd
}
