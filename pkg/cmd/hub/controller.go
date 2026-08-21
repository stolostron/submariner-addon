package hub

import (
	"context"
	"fmt"

	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"github.com/spf13/cobra"
	"github.com/stolostron/submariner-addon/pkg/hub"
	"github.com/stolostron/submariner-addon/pkg/version"
	opwebhook "github.com/submariner-io/submariner-operator/pkg/webhook"
	"k8s.io/klog/v2"
	"k8s.io/utils/clock"
	ctrl "sigs.k8s.io/controller-runtime"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func NewController() *cobra.Command {
	addOnOptions := hub.NewAddOnOptions()

	startFunc := func(ctx context.Context, controllerContext *controllercmd.ControllerContext) error {
		// Create controller-runtime manager for webhook support
		// Don't enable its health server - we'll use controllercmd's instead
		mgr, err := ctrl.NewManager(controllerContext.KubeConfig, ctrl.Options{
			Metrics: metricsserver.Options{
				BindAddress: "0", // Disable metrics
			},
			WebhookServer: webhook.NewServer(webhook.Options{
				Port:    9443,
				CertDir: "/tmp/k8s-webhook-server/serving-certs",
			}),
		})
		if err != nil {
			return fmt.Errorf("unable to create manager: %w", err)
		}

		// Register the broker webhook
		brokerValidator := opwebhook.NewBrokerValidator()
		brokerValidator.SetupWithManager(mgr)

		// Start the controller-runtime manager in background
		go func() {
			klog.Info("Starting controller-runtime manager for webhook server...")

			if err := mgr.Start(ctx); err != nil {
				klog.Fatalf("Controller-runtime manager failed: %v", err)
			}
		}()

		return addOnOptions.RunControllerManager(ctx, controllerContext)
	}

	cmd := controllercmd.
		NewControllerCommandConfig("submariner-controller", version.Get(), startFunc, clock.RealClock{}).
		NewCommand()
	cmd.Use = "controller"
	cmd.Short = "Start the ACM Submariner Controller"

	addOnOptions.AddFlags(cmd)

	return cmd
}
