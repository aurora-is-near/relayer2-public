module github.com/aurora-is-near/relayer2-public

go 1.18

//replace github.com/aurora-is-near/relayer2-base => github.com/aurora-is-near/relayer2-base v1.1.3-0.20230914105446-42f23496919e
//replace github.com/aurora-is-near/near-api-go => github.com/aurora-is-near/near-api-go

// The following package had a conflicting dependency.
// Fixed by pointing the dependency to the latest version tag.
replace github.com/btcsuite/btcd => github.com/btcsuite/btcd v0.23.2

require (
	github.com/aurora-is-near/near-api-go v0.0.14
	github.com/aurora-is-near/relayer2-base v1.1.5
	github.com/btcsuite/btcutil v1.0.2
	github.com/buger/jsonparser v1.1.1
	github.com/ethereum/go-ethereum v1.10.25
	github.com/google/uuid v1.3.0
	github.com/json-iterator/go v1.1.12
	github.com/spf13/cobra v1.6.0
	github.com/spf13/viper v1.13.0
	github.com/stretchr/testify v1.8.0
	github.com/valyala/fasthttp v1.47.0
	golang.org/x/net v0.17.0
)

require (
	capnproto.org/go/capnp/v3 v3.0.0-alpha.7 // indirect
	github.com/adhityaramadhanus/fasthttpcors v0.0.0-20170121111917-d4c07198763a // indirect
	github.com/andybalholm/brotli v1.0.5 // indirect
	github.com/aurora-is-near/go-jsonrpc/v3 v3.1.2 // indirect
	github.com/aurora-is-near/stream-backup v0.0.0-20221212013533-1e06e263c3f7 // indirect
	github.com/carlmjohnson/versioninfo v0.22.4 // indirect
	github.com/cespare/xxhash v1.1.0 // indirect
	github.com/dgraph-io/badger/v3 v3.2103.2 // indirect
	github.com/dgraph-io/ristretto v0.1.1 // indirect
	github.com/dustin/go-humanize v1.0.0 // indirect
	github.com/fasthttp/websocket v1.5.2 // indirect
	github.com/fxamacker/cbor/v2 v2.4.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/glog v1.0.0 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/flatbuffers v22.9.29+incompatible // indirect
	github.com/holiman/uint256 v1.2.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20200714003250-2b9c44734f2b // indirect
	github.com/jackc/pgx/v5 v5.2.0 // indirect
	github.com/jackc/puddle/v2 v2.1.2 // indirect
	github.com/klauspost/compress v1.16.3 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.16 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/near/borsh-go v0.3.1 // indirect
	github.com/pelletier/go-toml/v2 v2.0.5 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/puzpuzpuz/xsync/v2 v2.4.0 // indirect
	github.com/rs/zerolog v1.28.0 // indirect
	github.com/savsgio/gotils v0.0.0-20230208104028-c358bd845dee // indirect
	github.com/subosito/gotenv v1.4.1 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/tcplisten v1.0.0 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/yusufpapurcu/wmi v1.2.2 // indirect
	go.opencensus.io v0.23.0 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	golang.org/x/sync v0.1.0 // indirect
	google.golang.org/protobuf v1.30.0 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.0.0 // indirect
)

require (
	github.com/btcsuite/btcd/btcec/v2 v2.2.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.1.0 // indirect
	github.com/fsnotify/fsnotify v1.6.0
	github.com/go-stack/stack v1.8.1 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/inconshreveable/mousetrap v1.0.1 // indirect
	github.com/magiconair/properties v1.8.6 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/pelletier/go-toml v1.9.5 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/shirou/gopsutil v3.21.11+incompatible // indirect
	github.com/spf13/afero v1.9.2 // indirect
	github.com/spf13/cast v1.5.0 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/tklauser/go-sysconf v0.3.10 // indirect
	github.com/tklauser/numcpus v0.5.0 // indirect
	golang.org/x/crypto v0.14.0 // indirect
	golang.org/x/sys v0.13.0 // indirect
	golang.org/x/text v0.13.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
