package spoke

import (
	"github.com/spf13/cobra"
	"github.com/stolostron/submariner-addon/pkg/spoke"
	ctrl "sigs.k8s.io/controller-runtime"
)

func NewAgent() *cobra.Command {
	agentOptions := spoke.NewAgentOptions()

	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Start the ACM Submariner Agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			return agentOptions.RunAgent(ctrl.SetupSignalHandler(), ctrl.GetConfigOrDie())
		},
	}

	agentOptions.AddFlags(cmd)

	return cmd
}
