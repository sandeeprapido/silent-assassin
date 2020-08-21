package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/roppenlabs/silent-assassin/pkg/config"
	"github.com/roppenlabs/silent-assassin/pkg/gcloud"
	"github.com/roppenlabs/silent-assassin/pkg/k8s"
	"github.com/roppenlabs/silent-assassin/pkg/killer"
	"github.com/roppenlabs/silent-assassin/pkg/logger"
	"github.com/roppenlabs/silent-assassin/pkg/notifier"
	"github.com/roppenlabs/silent-assassin/pkg/server"
	"github.com/roppenlabs/silent-assassin/pkg/spotter"
	"github.com/spf13/cobra"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "starts silent assassin server",

	Long: ``,
	Run: func(cmd *cobra.Command, args []string) {
		sigChan := make(chan os.Signal)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		wg := &sync.WaitGroup{}
		ctx, cancelFn := context.WithCancel(context.Background())

		configProvider := config.Init(cfgFile)
		zapLogger := logger.Init(configProvider)
		kubeClient := k8s.NewClient(configProvider, zapLogger)
		gcloudClient, err := gcloud.NewClient()

		if err != nil {
			zapLogger.Error(fmt.Sprintf("Error creating gcloud client: %s", err.Error()))
		}

		ns := notifier.NewNotifier(configProvider, zapLogger)
		wg.Add(1)
		go ns.Start(ctx, wg)

		ss := spotter.NewSpotterService(configProvider, zapLogger, kubeClient, ns)
		wg.Add(1)
		go ss.Start(ctx, wg)

		ks := killer.NewKillerService(configProvider, zapLogger, kubeClient, gcloudClient, ns)
		wg.Add(1)
		go ks.Start(ctx, wg)

		server := server.NewHttpServer(configProvider, zapLogger, ks)
		wg.Add(1)
		go server.Start(ctx, wg)

		<-sigChan

		zapLogger.Info("Starting shut down")
		cancelFn()
		wg.Wait()
		zapLogger.Info("Shut down completed successfully")
	},
}

func init() {
	startCmd.AddCommand(serverCmd)
}
