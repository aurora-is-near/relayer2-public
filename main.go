package main

import (
	"fmt"
	"os"
	"os/signal"
	"relayer2-base/cmd"
	"relayer2-base/db"
	"relayer2-base/db/badger"
	commonEndpoint "relayer2-base/endpoint"
	"relayer2-base/endpoint/processor"
	"relayer2-base/indexer/prehistory"
	"relayer2-base/indexer/tar"
	"relayer2-base/log"
	goEthereum "relayer2-base/rpcnode/github-ethereum-go-ethereum"
	"relayer2-public/endpoint"
	"relayer2-public/indexer"
	"relayer2-public/middleware"
	"syscall"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func main() {
	c := cmd.RootCmd()
	c.AddCommand(cmd.VersionCmd())
	c.AddCommand(cmd.StartCmd(func(cmd *cobra.Command, args []string) {

		logger := log.Log()
		bh, err := badger.NewBlockHandler()
		if err != nil {
			logger.Fatal().Err(err).Msg("failed to start block handler")
		}
		fh, err := badger.NewFilterHandler()
		if err != nil {
			logger.Fatal().Err(err).Msg("failed to start filter handler")
		}

		handler := db.StoreHandler{
			BlockHandler:  bh,
			FilterHandler: fh,
		}
		defer handler.Close()

		baseEndpoint := commonEndpoint.New(handler)

		// order of processors are important
		baseEndpoint.WithProcessor(processor.NewEnableDisable())
		baseEndpoint.WithProcessor(processor.NewProxy())

		ethEndpoint := commonEndpoint.NewEth(baseEndpoint)
		netEndpoint := commonEndpoint.NewNet(baseEndpoint)
		web3Endpoint := commonEndpoint.NewWeb3(baseEndpoint)
		parityEnpoint := commonEndpoint.NewParity(baseEndpoint)
		debugEnpoint := commonEndpoint.NewDebug(baseEndpoint)

		rpcNode, err := goEthereum.New()
		if err != nil {
			logger.Fatal().Err(err).Msg("failed to create json-rpc server")
		}

		var rpcAPIs []rpc.API
		rpcAPIs = append(rpcAPIs, rpc.API{
			Namespace: "eth",
			Version:   "1.0",
			Service:   commonEndpoint.NewEthProcessorAware(ethEndpoint),
		})
		rpcAPIs = append(rpcAPIs, rpc.API{
			Namespace: "net",
			Version:   "1.0",
			Service:   commonEndpoint.NewNetProcessorAware(netEndpoint),
		})
		rpcAPIs = append(rpcAPIs, rpc.API{
			Namespace: "web3",
			Version:   "1.0",
			Service:   commonEndpoint.NewWeb3ProcessorAware(web3Endpoint),
		})
		rpcAPIs = append(rpcAPIs, rpc.API{
			Namespace: "parity",
			Version:   "1.0",
			Service:   commonEndpoint.NewParityProcessorAware(parityEnpoint),
		})
		rpcAPIs = append(rpcAPIs, rpc.API{
			Namespace: "debug",
			Version:   "1.0",
			Service:   commonEndpoint.NewDebugProcessorAware(debugEnpoint),
		})

		eventsEndpoint := endpoint.NewEventsForGoEth(baseEndpoint, rpcNode.Broker)
		rpcAPIs = append(rpcAPIs, rpc.API{
			Namespace: "eth",
			Version:   "1.0",
			Service:   eventsEndpoint,
		})

		engineEth := endpoint.NewEngineEth(baseEndpoint)
		rpcAPIs = append(rpcAPIs, rpc.API{
			Namespace: "eth",
			Version:   "1.0",
			Service:   engineEth,
		})

		rpcAPIs = append(rpcAPIs, rpc.API{
			Namespace: "net",
			Version:   "1.0",
			Service:   endpoint.NewEngineNet(engineEth),
		})

		rpcNode.RegisterAPIs(rpcAPIs)

		rpcNode.WithMiddleware("filterIP", "/", middleware.FilterIP)

		err = rpcNode.Start()
		if err != nil {
			logger.Fatal().Err(err).Msg("failed to start json-rpc server")
		}
		// Stop geth's p2p server
		rpcNode.Server().Stop()
		defer rpcNode.Close()

		var tarIndexer *tar.Indexer
		tarIndexer, err = tar.New(handler)
		if err != nil {
			logger.Fatal().Err(err).Msg("failed to start tar indexer")
		}
		if tarIndexer != nil {
			tarIndexer.Start()
			defer tarIndexer.Close()
		}

		var indxr *indexer.Indexer
		if rpcNode.Broker != nil {
			indxr, err = indexer.NewWithBroker(handler, rpcNode.Broker)
		} else {
			indxr, err = indexer.New(handler)
		}
		if err != nil {
			logger.Fatal().Err(err).Msg("failed to start indexer")
		}
		indxr.Start()
		defer indxr.Close()

		preIndxr, err := prehistory.New(handler)
		if err != nil {
			logger.Fatal().Err(err).Msg("failed to start indexer")
		}
		if preIndxr != nil {
			preIndxr.Start()
			defer preIndxr.Close()
		}

		// Set the handlers for components that needs to updated after config changes
		viper.OnConfigChange(func(e fsnotify.Event) {
			logger.HandleConfigChange()
			rpcNode.HandleConfigChange()
			baseEndpoint.HandleConfigChange()
		})

		sig := make(chan os.Signal)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
	}))

	if err := c.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "There was an error while executing Relayer CLI '%s'", err)
		os.Exit(1)
	}
}
