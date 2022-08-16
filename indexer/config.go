package indexer

const (
	DefaultJsonFilesPath = "/tmp/relayer/json/"
)

type Config struct {
	JsonFilesPath string
}

func DefaultConfig() Config {
	return Config{JsonFilesPath: DefaultJsonFilesPath}
}
