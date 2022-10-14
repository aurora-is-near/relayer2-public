package main

import (
	"aurora-relayer-go-common/cmd"
	"aurora-relayer-go-common/db"
	"aurora-relayer-go-common/db/badger"
	commonEndpoint "aurora-relayer-go-common/endpoint"
	"aurora-relayer-go-common/endpoint/processor"
	"aurora-relayer-go-common/log"
	goEthereum "aurora-relayer-go-common/rpcnode/github-ethereum-go-ethereum"
	"aurora-relayer-go/endpoint"
	"aurora-relayer-go/indexer"
	"aurora-relayer-go/middleware"
	"os"
	"os/signal"
	"syscall"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/spf13/cobra"
)

const (
	version = "0.1.0"
)

func main() {

	c := cmd.RootCmd()
	c.AddCommand(cmd.VersionCmd(func(cmd *cobra.Command, args []string) {
		println("app: ", "v", version)
	}))

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
		baseEndpoint.WithProcessor(processor.NewBadgerTxn())

		ethEndpoint := commonEndpoint.NewEth(baseEndpoint)
		web3Endpoint := commonEndpoint.NewWeb3(baseEndpoint)

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
			Namespace: "web3",
			Version:   "1.0",
			Service:   commonEndpoint.NewWeb3ProcessorAware(web3Endpoint),
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
		defer rpcNode.Close()

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

		sig := make(chan os.Signal)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
	}))

	if err := c.Execute(); err != nil {
		os.Exit(1)
	}
}
