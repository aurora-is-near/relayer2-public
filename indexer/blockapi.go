package indexer

import (
	"context"
	"errors"
	"sync"
	"time"

	blocksapi "github.com/aurora-is-near/borealis-prototypes-go/generated/blocksapi"
	"github.com/aurora-is-near/relayer2-base/broker"
	"github.com/aurora-is-near/relayer2-base/db"
	"github.com/aurora-is-near/relayer2-base/log"
	"github.com/aurora-is-near/relayer2-base/types"
	"github.com/aurora-is-near/relayer2-base/types/common"
	"github.com/aurora-is-near/relayer2-base/types/indexer"
	"github.com/aurora-is-near/relayer2-base/utils"

	vtgrpc "github.com/planetscale/vtprotobuf/codec/grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/encoding"
	_ "google.golang.org/grpc/encoding/proto"
	"google.golang.org/grpc/metadata"
)

const logTickerDuration = 5 * time.Second

type IndexerBlocksAPI struct {
	token  string
	stream string

	dbh          db.Handler
	l            *log.Logger
	config       *Config
	b            broker.Broker
	grpc         *grpc.ClientConn
	nextHeight   uint64
	mu           sync.Mutex
	cancelRunCtx context.CancelFunc
	wg           sync.WaitGroup
}

func init() {
	// Register protobuf plugin for generate optimized marshall & unmarshal code
	encoding.RegisterCodec(vtgrpc.Codec{})
}

func NewIndexerBlocksApi(config *Config, dbh db.Handler, b broker.Broker) (*IndexerBlocksAPI, error) {
	logger := log.Log()

	client, err := grpc.NewClient(
		// TODO: use client-side load balancer
		config.BlocksApiUrl[0],
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithInitialConnWindowSize(64*1024*1024),                           // 64 MB
		grpc.WithInitialWindowSize(64*1024*1024),                               // 64 MB
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(1*1024*1024*1024)), // 1GB
	)
	if err != nil {
		return nil, err
	}

	return &IndexerBlocksAPI{
		stream:     config.BlocksApiStream,
		token:      config.BlocksApiToken,
		dbh:        dbh,
		l:          logger,
		config:     config,
		b:          b,
		grpc:       client,
		nextHeight: config.FromBlock,
	}, nil
}

func (i *IndexerBlocksAPI) Start(ctx context.Context) {
	i.mu.Lock()
	defer i.mu.Unlock()

	// Attaching authorization token
	md := metadata.New(make(map[string]string))
	md.Set("authorization", "Bearer "+i.token)

	runCtx, cancel := context.WithCancel(ctx)
	callCtx := metadata.NewOutgoingContext(runCtx, md)

	i.cancelRunCtx = cancel
	i.wg.Add(1)

	go func() {
		defer i.wg.Done()

		for {
			if err := i.run(callCtx); err != nil {
				// If there's an error, we wait a bit before trying again, or
				// until context is closed
				select {
				case <-callCtx.Done():
					return
				case <-time.After(time.Second):
					continue
				}
			}

			if !i.config.ForceReindex && i.config.ToBlock != 0 {
				// We're done
				i.l.Info().Msg("Indexer is done")
				return
			}

			lb, err := i.dbh.BlockNumber(context.Background())
			if err != nil || lb == nil {
				i.l.Fatal().Err(err).
					Msg("Failed to read the latest block after re-indexing")
				return
			}

			fb := uint64(*lb)
			i.l.Info().
				Uint64("from_block", i.config.FromBlock).
				Uint64("to_block", i.config.ToBlock).
				Uint64("cur_block", fb).
				Msg("Re-indexing finished, indexing will continue from current block")

			// We reached the target block, but we want to keep going
			i.config.ToBlock = 0
			i.nextHeight = fb
		}
	}()
}

func (i *IndexerBlocksAPI) run(callCtx context.Context) error {
	blocksProviderClient := blocksapi.NewBlocksProviderClient(i.grpc)

	request := &blocksapi.ReceiveBlocksRequest{
		StreamName:  i.stream,
		StartPolicy: blocksapi.ReceiveBlocksRequest_START_EXACTLY_ON_TARGET,
		StopPolicy:  blocksapi.ReceiveBlocksRequest_STOP_NEVER,
		StartTarget: &blocksapi.BlockMessage_ID{
			Kind:   blocksapi.BlockMessage_MSG_WHOLE,
			Height: i.nextHeight,
		},
	}

	if i.config.ToBlock > 0 {
		request.StopPolicy = blocksapi.ReceiveBlocksRequest_STOP_AFTER_TARGET
		request.StopTarget = &blocksapi.BlockMessage_ID{
			Kind:   blocksapi.BlockMessage_MSG_WHOLE,
			Height: i.config.ToBlock,
		}
	}

	callClient, err := blocksProviderClient.ReceiveBlocks(callCtx, request)
	if err != nil {
		i.l.Error().Err(err).Msgf("unable to call ReceiveBlocks")
		return err
	}

	defer callClient.CloseSend()

	logTicker := time.NewTicker(logTickerDuration)
	var lastSpeedLogTime = time.Now()
	var msgsSinceLastSpeedLog uint64 = 0
	var bytesSinceLastSpeedLog uint64 = 0

	for {
		select {
		case <-callCtx.Done():
			logTicker.Stop()
			return nil
		case now := <-logTicker.C:
			timeDiff := time.Since(lastSpeedLogTime)
			msgsSpeed := float64(msgsSinceLastSpeedLog) / timeDiff.Seconds()
			mbSpeed := float64(bytesSinceLastSpeedLog) / timeDiff.Seconds() / 1024 / 1024

			i.l.Info().
				Float64("mps", msgsSpeed).
				Float64("mbps", mbSpeed).
				Msg("Indexing speed")

			lastSpeedLogTime = now
			msgsSinceLastSpeedLog = 0
			bytesSinceLastSpeedLog = 0
		default:
		}

		response, err := callClient.Recv()
		if err != nil {
			i.l.Error().Err(err).Msg("unable to receive next response")
			return err
		}

		switch r := response.Response.(type) {
		case *blocksapi.ReceiveBlocksResponse_Error_:
			i.l.Warn().Msgf("Got gRPC error: %v", r)
			return errors.New(r.Error.Description)
		case *blocksapi.ReceiveBlocksResponse_Done_:
			i.l.Info().Msg("Target block reached")
			return nil
		case *blocksapi.ReceiveBlocksResponse_Message:
			payload := r.Message.Message.GetRawPayload()
			if payload == nil {
				i.l.Fatal().Msg("invalid payload type")
				return err
			}

			block, err := DecodeBorealisPayload[indexer.Block](payload)
			if err != nil {
				i.l.Fatal().Err(err).Msg("couln't parse block")
				return err
			}

			err = i.dbh.InsertBlock(block)
			if err != nil {
				i.l.Fatal().Err(err).Msg("couln't insert block")
				return err
			}

			if i.b != nil {
				ctx := utils.PutChainId(callCtx, block.ChainId)
				bn := common.UintToBN64(block.Height)
				block, err := i.dbh.GetBlockByNumber(ctx, bn, true)
				if err != nil {
					// just log, this is a best-effort operation
					i.l.Error().Err(err).Msg("couln't get block number")
				} else {
					i.b.PublishNewHeads(block)
				}

				nfilter := &(types.Filter{FromBlock: bn.Uint64(), ToBlock: bn.Uint64()})
				logs, err := i.dbh.GetLogs(ctx, nfilter.ToLogFilter())
				if err != nil {
					// just log, this is a best-effort operation
					i.l.Error().Err(err).Msg("couln't get block logs")
				} else {
					i.b.PublishLogs(logs)
				}
			}

			i.nextHeight += 1

			msgsSinceLastSpeedLog++
			bytesSinceLastSpeedLog += uint64(len(payload))
		default:
			i.l.Warn().Msg("Unknown type")
		}
	}
}

func (i *IndexerBlocksAPI) Close() {
	i.cancelRunCtx()
	i.wg.Wait()

	if err := i.grpc.Close(); err != nil {
		log.Log().Printf("Unable to close connection: %v", err)
	}
}
