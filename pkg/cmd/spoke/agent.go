package spoke

import (
	"context"

	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"github.com/spf13/cobra"
	base "github.com/stolostron/submariner-addon/pkg/cmd"
	"github.com/stolostron/submariner-addon/pkg/spoke"
	"github.com/stolostron/submariner-addon/pkg/version"
)

func NewAgent() *cobra.Command {
	agentOptions := spoke.NewAgentOptions()
	cmdConfig := controllercmd.
		NewControllerCommandConfig("submariner-agent", version.Get(), agentOptions.RunAgent)
	cmd := cmdConfig.NewCommandWithContext(context.Background())
	cmd.Use = "agent"
	cmd.Short = "Start the ACM Submariner Agent"

	base.AddLeaderElectionFlags(cmd.Flags(), cmdConfig)
	agentOptions.AddFlags(cmd)

	return cmd
}
