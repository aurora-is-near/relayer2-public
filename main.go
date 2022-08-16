package main

import (
	"aurora-relayer-go-common/cmd"
	"aurora-relayer-go-common/db/badger"
	commonEndpoint "aurora-relayer-go-common/endpoint"
	goEthereum "aurora-relayer-go-common/rpcnode/github-ethereum-go-ethereum"
	"aurora-relayer-go/endpoint"
	"aurora-relayer-go/indexer"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/spf13/cobra"
	"os"
	"os/signal"
	"syscall"
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

		dbHandler, err := badger.New()
		if err != nil {
			os.Exit(1)
		}
		defer dbHandler.Close()

		indxr := indexer.New(dbHandler)
		err = indxr.Start()
		if err != nil {
			os.Exit(1)
		}
		defer indxr.Stop()

		baseEndpoint := commonEndpoint.New(&dbHandler)

		rpcNode, err := goEthereum.New()
		if err != nil {
			os.Exit(1)
		}

		var rpcAPIs []rpc.API
		rpcAPIs = append(rpcAPIs, rpc.API{
			Namespace: "eth",
			Version:   "1.0",
			Service:   commonEndpoint.NewEth(baseEndpoint),
		})
		rpcAPIs = append(rpcAPIs, rpc.API{
			Namespace: "web3",
			Version:   "1.0",
			Service:   commonEndpoint.NewWeb3(baseEndpoint),
		})
		rpcAPIs = append(rpcAPIs, rpc.API{
			Namespace: "eth",
			Version:   "1.0",
			Service:   endpoint.NewCustomEth(baseEndpoint),
		})
		rpcNode.RegisterAPIs(rpcAPIs)
		rpcNode.Start()
		defer rpcNode.Close()

		sig := make(chan os.Signal)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
	}))

	if err := c.Execute(); err != nil {
		os.Exit(1)
	}
}
