package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/aurora-is-near/relayer2-base/cmd"
	"github.com/aurora-is-near/relayer2-base/db"
	"github.com/aurora-is-near/relayer2-base/db/badger"
	commonEndpoint "github.com/aurora-is-near/relayer2-base/endpoint"
	"github.com/aurora-is-near/relayer2-base/endpoint/processor"
	"github.com/aurora-is-near/relayer2-base/indexer/prehistory"
	"github.com/aurora-is-near/relayer2-base/indexer/tar"
	"github.com/aurora-is-near/relayer2-base/log"
	"github.com/aurora-is-near/relayer2-base/rpc/node"
	"github.com/aurora-is-near/relayer2-public/endpoint"
	"github.com/aurora-is-near/relayer2-public/indexer"
	"github.com/aurora-is-near/relayer2-public/middleware"

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

		// create json-rpc node
		rpcNode, err := node.New()
		if err != nil {
			logger.Fatal().Err(err).Msg("failed to create json-rpc server")
		}

		baseEndpoint := commonEndpoint.New(handler)
		// order of processors are important
		baseEndpoint.WithProcessor(processor.NewEnableDisable())

		ethEndpoint := commonEndpoint.NewEth(baseEndpoint)
		ethProcessor := commonEndpoint.NewEthProcessorAware(ethEndpoint)
		netEndpoint := commonEndpoint.NewNet(baseEndpoint)
		netProcessor := commonEndpoint.NewNetProcessorAware(netEndpoint)
		web3Endpoint := commonEndpoint.NewWeb3(baseEndpoint)
		web3Processor := commonEndpoint.NewWeb3ProcessorAware(web3Endpoint)
		parityEndpoint := commonEndpoint.NewParity(baseEndpoint)
		parityProcessor := commonEndpoint.NewParityProcessorAware(parityEndpoint)
		debugEndpoint := commonEndpoint.NewDebug(baseEndpoint)
		debugProcessor := commonEndpoint.NewDebugProcessorAware(debugEndpoint)
		eventsEndpoint := endpoint.NewEventsEth(baseEndpoint, rpcNode.Broker)
		engineEth := endpoint.NewEngineEth(baseEndpoint)
		engineNet := endpoint.NewEngineNet(engineEth)
		defer engineEth.Close()

		// use the endpoint services to json-rpc server
		rpcNode.WithMiddleware(middleware.FilterIP())
		rpcNode.WithMiddleware(middleware.Proxy(baseEndpoint))

		// register endpoints and events to json-rpc server
		err = rpcNode.RegisterEndpoints("eth", ethProcessor)
		if err != nil {
			logger.Fatal().Err(err).Msg("failed to register `eth` endpoints")
		}
		err = rpcNode.RegisterEndpoints("net", netProcessor)
		if err != nil {
			logger.Fatal().Err(err).Msg("failed to register `net` endpoints")
		}
		err = rpcNode.RegisterEndpoints("web3", web3Processor)
		if err != nil {
			logger.Fatal().Err(err).Msg("failed to register `web3` endpoints")
		}
		err = rpcNode.RegisterEndpoints("parity", parityProcessor)
		if err != nil {
			logger.Fatal().Err(err).Msg("failed to register `parity` endpoints")
		}
		err = rpcNode.RegisterEndpoints("debug", debugProcessor)
		if err != nil {
			logger.Fatal().Err(err).Msg("failed to register `debug` endpoints")
		}
		err = rpcNode.RegisterEndpoints("eth", engineEth)
		if err != nil {
			logger.Fatal().Err(err).Msg("failed to register `eth` endpoints for engine")
		}
		err = rpcNode.RegisterEndpoints("net", engineNet)
		if err != nil {
			logger.Fatal().Err(err).Msg("failed to register `net` endpoints for engine")
		}

		err = rpcNode.RegisterEvents("eth", eventsEndpoint)
		if err != nil {
			logger.Fatal().Err(err).Msg("failed to register `eth` events")
		}
		rpcNode.Start()
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
