package hub

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"github.com/openshift/library-go/pkg/serviceability"
	"github.com/spf13/cobra"
	"github.com/stolostron/submariner-addon/pkg/hub"
	"github.com/stolostron/submariner-addon/pkg/resource"
	"github.com/stolostron/submariner-addon/pkg/version"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

func NewController() *cobra.Command {
	addOnOptions := hub.NewAddOnOptions()

	cmd := &cobra.Command{
		Use:   "controller",
		Short: "Start the ACM Submariner Controller",
		RunE: func(cmd *cobra.Command, args []string) error {
			return startManager(ctrl.SetupSignalHandler(), addOnOptions)
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

	runnable := &addonControllerRunnable{
		config:       cfg,
		addOnOptions: addOnOptions,
	}

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		LeaderElection:                true,
		LeaderElectionID:              "submariner-controller-lock",
		LeaderElectionNamespace:       resource.GetCurrentNamespace(hub.DefaultNamespace),
		LeaderElectionReleaseOnCancel: true,
		LeaseDuration:                 &leaseDuration,
		RenewDeadline:                 &renewDeadline,
		RetryPeriod:                   &retryPeriod,
		HealthProbeBindAddress:        ":8081",
		Metrics: metricsserver.Options{
			BindAddress: "0",
		},
	})
	if err != nil {
		return fmt.Errorf("unable to create manager: %w", err)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return fmt.Errorf("unable to add healthz check: %w", err)
	}

	// Custom readyz check that waits for informer caches to sync
	if err := mgr.AddReadyzCheck("informers", runnable.readyzCheck); err != nil {
		return fmt.Errorf("unable to add readyz check: %w", err)
	}

	if err := mgr.Add(runnable); err != nil {
		return fmt.Errorf("unable to add addon controller runnable: %w", err)
	}

	defer serviceability.BehaviorOnPanic(os.Getenv("OPENSHIFT_ON_PANIC"), version.Get())()
	defer serviceability.Profile(os.Getenv("OPENSHIFT_PROFILE")).Stop()

	serviceability.StartProfiler()

	return mgr.Start(ctx) //nolint:wrapcheck // No need to wrap
}

// addonControllerRunnable wraps the addon framework startup so it runs under
// controller-runtime's manager, gated by leader election.
type addonControllerRunnable struct {
	config       *rest.Config
	addOnOptions *hub.AddOnOptions
	ready        atomic.Bool
}

func (r *addonControllerRunnable) Start(ctx context.Context) error {
	return r.addOnOptions.RunControllerManager(ctx, r.config, r.markReady) //nolint:wrapcheck // No need to wrap
}

// NeedLeaderElection ensures the addon controllers only run on the elected leader.
func (r *addonControllerRunnable) NeedLeaderElection() bool {
	return true
}

// markReady is called by RunControllerManager after informer caches are synced.
func (r *addonControllerRunnable) markReady() {
	r.ready.Store(true)
}

// readyzCheck is the health check for readiness - waits for informer caches to sync.
func (r *addonControllerRunnable) readyzCheck(_ *http.Request) error {
	if r.ready.Load() {
		return nil
	}

	return errors.New("informer caches not yet synced")
}
