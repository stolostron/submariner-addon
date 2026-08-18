package hub

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	openshifttls "github.com/openshift/controller-runtime-common/pkg/tls"
	"github.com/openshift/library-go/pkg/serviceability"
	"github.com/spf13/cobra"
	"github.com/stolostron/submariner-addon/pkg/hub"
	"github.com/stolostron/submariner-addon/pkg/resource"
	"github.com/stolostron/submariner-addon/pkg/version"
	opwebhook "github.com/submariner-io/submariner-operator/pkg/webhook"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
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
	utilruntime.Must(configv1.Install(scheme.Scheme))

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	cfg := ctrl.GetConfigOrDie()

	tlsProfile, err := getTLSProfile(ctx, cfg)
	if err != nil {
		return err
	}

	tlsConfigFunc, _ := openshifttls.NewTLSConfigFromProfile(tlsProfile)

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
		WebhookServer: webhook.NewServer(webhook.Options{
			Port:    9443,
			CertDir: "/tmp/k8s-webhook-server/serving-certs",
			TLSOpts: []func(*tls.Config){tlsConfigFunc},
		}),
	})
	if err != nil {
		return fmt.Errorf("unable to create manager: %w", err)
	}

	brokerValidator := opwebhook.NewBrokerValidator()
	brokerValidator.SetupWithManager(mgr)

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return fmt.Errorf("unable to add healthz check: %w", err)
	}

	if err := mgr.AddReadyzCheck("readyz", mgr.GetWebhookServer().StartedChecker()); err != nil {
		return fmt.Errorf("unable to add readyz check: %w", err)
	}

	if err := mgr.Add(runnable); err != nil {
		return fmt.Errorf("unable to add addon controller runnable: %w", err)
	}

	tlsProfileWatcher := &openshifttls.SecurityProfileWatcher{
		Client:                mgr.GetClient(),
		InitialTLSProfileSpec: tlsProfile,
		OnProfileChange: func(_ context.Context, oldProfile, newProfile configv1.TLSProfileSpec) {
			klog.Infof("TLS security profile changed. Old: MinVersion=%s Ciphers=%d, New: MinVersion=%s Ciphers=%d",
				oldProfile.MinTLSVersion, len(oldProfile.Ciphers), newProfile.MinTLSVersion, len(newProfile.Ciphers))
			cancel()
		},
	}

	if err := tlsProfileWatcher.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("failed to setup TLS profile watcher: %w", err)
	}

	defer serviceability.BehaviorOnPanic(os.Getenv("OPENSHIFT_ON_PANIC"), version.Get())()
	defer serviceability.Profile(os.Getenv("OPENSHIFT_PROFILE")).Stop()

	serviceability.StartProfiler()

	return mgr.Start(ctx) //nolint:wrapcheck // No need to wrap
}

func getTLSProfile(ctx context.Context, cfg *rest.Config) (configv1.TLSProfileSpec, error) {
	crClient, err := client.New(cfg, client.Options{})
	if err != nil {
		return configv1.TLSProfileSpec{}, fmt.Errorf("error creating client: %w", err)
	}

	return openshifttls.FetchAPIServerTLSProfile(ctx, crClient) //nolint:wrapcheck // No need to wrap
}

// addonControllerRunnable wraps the addon framework startup so it runs under
// controller-runtime's manager, gated by leader election.
type addonControllerRunnable struct {
	config       *rest.Config
	addOnOptions *hub.AddOnOptions
}

func (r *addonControllerRunnable) Start(ctx context.Context) error {
	return r.addOnOptions.RunControllerManager(ctx, r.config, nil) //nolint:wrapcheck // No need to wrap
}

// NeedLeaderElection ensures the addon controllers only run on the elected leader.
func (r *addonControllerRunnable) NeedLeaderElection() bool {
	return true
}
