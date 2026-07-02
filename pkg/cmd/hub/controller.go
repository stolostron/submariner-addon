package hub

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/stolostron/submariner-addon/pkg/hub"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
)

func NewController() *cobra.Command {
	addOnOptions := hub.NewAddOnOptions()

	cmd := &cobra.Command{
		Use:   "controller",
		Short: "Start the ACM Submariner Controller",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := ctrl.SetupSignalHandler()
			return startManager(ctx, addOnOptions)
		},
	}

	addOnOptions.AddFlags(cmd)

	return cmd
}

func startManager(ctx context.Context, addOnOptions *hub.AddOnOptions) error {
	cfg := ctrl.GetConfigOrDie()

	leaseDuration := 137 * time.Second
	renewDeadline := 107 * time.Second
	retryPeriod := 26 * time.Second

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		LeaderElection:                true,
		LeaderElectionID:              "submariner-addon-controller-lock",
		LeaderElectionReleaseOnCancel: true,
		LeaseDuration:                 &leaseDuration,
		RenewDeadline:                 &renewDeadline,
		RetryPeriod:                   &retryPeriod,
		HealthProbeBindAddress:        ":8081",
		PprofBindAddress:              "127.0.0.1:6060", // Localhost only for security
	})
	if err != nil {
		return fmt.Errorf("unable to create manager: %w", err)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return fmt.Errorf("unable to add healthz check: %w", err)
	}

	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return fmt.Errorf("unable to add readyz check: %w", err)
	}

	if err := mgr.Add(&addonControllerRunnable{
		config:       cfg,
		addOnOptions: addOnOptions,
	}); err != nil {
		return fmt.Errorf("unable to add addon controller runnable: %w", err)
	}

	return mgr.Start(ctx) //nolint:wrapcheck // No need to wrap
}

// addonControllerRunnable wraps the addon framework startup so it runs under
// controller-runtime's manager, gated by leader election.
type addonControllerRunnable struct {
	config       *rest.Config
	addOnOptions *hub.AddOnOptions
}

func (r *addonControllerRunnable) Start(ctx context.Context) error {
	return r.addOnOptions.RunControllerManager(ctx, r.config) //nolint:wrapcheck // No need to wrap
}

// NeedLeaderElection ensures the addon controllers only run on the elected leader.
func (r *addonControllerRunnable) NeedLeaderElection() bool {
	return true
}
