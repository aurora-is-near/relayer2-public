package indexer

import (
	"context"
	"errors"
	"io"
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

	dbh        db.Handler
	l          *log.Logger
	b          broker.Broker
	grpc       *grpc.ClientConn
	nextHeight uint64
	mu         sync.Mutex
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

	callCtx := metadata.NewOutgoingContext(ctx, md)

	go i.run(callCtx)
}

func (i *IndexerBlocksAPI) run(callCtx context.Context) {
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

	callClient, err := blocksProviderClient.ReceiveBlocks(callCtx, request)
	if err != nil {
		i.l.Error().Err(err).Msgf("unable to call ReceiveBlocks")
		return
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
			return
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
		if errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			i.l.Error().Err(err).Msg("unable to receive next response")
			return
		}

		switch r := response.Response.(type) {
		case *blocksapi.ReceiveBlocksResponse_Error_:
			i.l.Warn().Msgf("Got gRPC error: %v", r)
		case *blocksapi.ReceiveBlocksResponse_Message:
			payload := r.Message.Message.GetRawPayload()
			if payload == nil {
				i.l.Fatal().Msg("invalid payload type")
				return
			}

			block, err := DecodeBorealisPayload[indexer.Block](payload)
			if err != nil {
				i.l.Fatal().Err(err).Msg("couln't parse block")
				return
			}

			err = i.dbh.InsertBlock(block)
			if err != nil {
				i.l.Fatal().Err(err).Msg("couln't insert block")
				return
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
	if err := i.grpc.Close(); err != nil {
		log.Log().Printf("Unable to close connection: %v", err)
	}
}
