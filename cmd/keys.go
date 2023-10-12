package cmd

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aurora-is-near/near-api-go"
	"github.com/aurora-is-near/near-api-go/keystore"
	"github.com/aurora-is-near/near-api-go/utils"
	"github.com/btcsuite/btcutil/base58"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var defaultConfigFile = "config/testnet.yaml"
var numberOfKeys uint64
var prefix = "ed25519:"

type KeyFile struct {
	AccountID string `json:"account_id"`
	PublicKey string `json:"public_key"`
	SecretKey string `json:"secret_key"`
}

type Config struct {
	Signer                   string `mapstructure:"signer"`
	SignerKey                string `mapstructure:"signerKey"`
	FunctionKeyPrefixPattern string `mapstructure:"functionKeyPrefixPattern"`
	NearNetworkID            string `mapstructure:"networkID"`
	NearNodeURL              string `mapstructure:"nearNodeURL"`
	NearArchivalNodeURL      string `mapstructure:"nearArchivalNodeURL"`
	NearConfig               near.Config
}

// GenerateKeysCmd generates a new key pair and attempts to save it to the file
func GenerateKeysCmd() *cobra.Command {
	generateKey := &cobra.Command{
		Use:   "generate-key",
		Short: "Command to generate signer key pair",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return bindConfiguration(cmd)
		},

		RunE: func(cmd *cobra.Command, args []string) error {
			config := loadConfig()
			key := newKey("")
			if config.SignerKey == "" {
				fmt.Println("signerKey is not set, please provide it in the config file")
				os.Exit(1)
			}
			dumpJson(config.SignerKey, key)
			fmt.Printf("key generated: [%s]", config.SignerKey)
			return nil
		},
	}
	acceptConfigFilePath(generateKey)
	return generateKey
}

// AddKeysCmd adds keys to the account via batch transaction
func AddKeysCmd() *cobra.Command {
	addKeys := &cobra.Command{
		Use:   "add-keys",
		Short: "Command to add keys to the account",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return bindConfiguration(cmd)
		},

		RunE: func(cmd *cobra.Command, args []string) error {
			config := loadConfig()

			if numberOfKeys == 0 {
				fmt.Println("number-of-keys is not set, please provide it using -n flag")
				os.Exit(1)
			}

			destDir := filepath.Dir(config.SignerKey)
			baseName := filepath.Base(config.SignerKey)

			nearConn := near.NewConnection(config.NearConfig.NodeURL)
			nearAccount, err := near.LoadAccount(nearConn, &config.NearConfig, config.Signer)
			if err != nil {
				fmt.Printf("failed to load Near account from path: [%s](%s)", config.SignerKey, err)
				os.Exit(1)
			}
			pubKeys := make([]utils.PublicKey, 0)

			for i := uint64(0); i < numberOfKeys; i++ {
				key := newKey(config.Signer)
				pubKeys = append(pubKeys, publicKeyFromBase58(key.PublicKey))
				filename := fmt.Sprintf("%s/fk%v.%s", destDir, i, baseName)
				err = dumpJson(filename, key)
				if err != nil {
					fmt.Printf("failed to dump key to file: [%s](%s) \n", filename, err)
					os.Exit(1)
				}
			}
			result, err := nearAccount.AddKeys(pubKeys...)
			if err != nil {
				fmt.Printf("failed to add key: [%s]", err)
				os.Exit(1)
			}
			status := result["status"].(map[string]interface{})
			if status["Failure"] != nil {
				fmt.Printf("failed to add key: [%s]", status["Failure"])
				os.Exit(1)
			}

			fmt.Printf("%v keys where added", len(pubKeys))

			return nil
		},
	}
	acceptConfigFilePath(addKeys)
	addKeys.PersistentFlags().Uint64VarP(&numberOfKeys, "number-of-keys", "n", 0, "Amount of access keys to generate and add to the account")
	return addKeys
}

// DeleteKeysCmd deletes keys from the account via batch transaction
func DeleteKeysCmd() *cobra.Command {
	deleteKeys := &cobra.Command{
		Use:   "delete-keys",
		Short: "Command to delete keys from the account",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return bindConfiguration(cmd)
		},

		RunE: func(cmd *cobra.Command, args []string) error {
			config := loadConfig()

			keyPairFilePaths := getFunctionCallKeyPairFilePaths(config.SignerKey, config.FunctionKeyPrefixPattern)
			nearConn := near.NewConnection(config.NearConfig.NodeURL)
			nearAccount, err := near.LoadAccount(nearConn, &config.NearConfig, config.Signer)
			if err != nil {
				fmt.Printf("failed to load Near account from path: [%s](%s)", config.SignerKey, err)
				os.Exit(1)
			}

			pubKeys := make([]utils.PublicKey, 0)
			for _, keyPairFilePath := range keyPairFilePaths {
				keyPair, err := keystore.LoadKeyPairFromPath(keyPairFilePath, config.Signer)
				if err != nil {
					fmt.Printf("failed to load key pair from path: [%s](%s)", keyPairFilePath, err)
					os.Exit(1)
				}
				pubKeys = append(pubKeys, publicKeyFromBase58(keyPair.PublicKey))
			}

			result, err := nearAccount.DeleteKeys(pubKeys...)
			if err != nil {
				fmt.Printf("failed to delete key: [%s]", err)
				os.Exit(1)
			}
			status := result["status"].(map[string]interface{})
			if status["Failure"] != nil {
				fmt.Printf("failed to delete key: [%s]", status["Failure"])
				os.Exit(1)
			}
			for _, keyPairFilePath := range keyPairFilePaths {
				err := os.Remove(keyPairFilePath)
				if err != nil {
					fmt.Printf("failed to remove key pair file: [%s](%s)", keyPairFilePath, err)
					os.Exit(1)
				}
			}

			fmt.Printf("%v keys where deleted", len(keyPairFilePaths))

			os.Exit(0)

			return nil
		},
	}
	acceptConfigFilePath(deleteKeys)
	return deleteKeys
}

func bindConfiguration(cmd *cobra.Command) error {
	configFile, _ := cmd.Flags().GetString("config")
	if configFile != "" {
		viper.SetConfigFile(configFile)
	} else {
		viper.SetConfigFile(defaultConfigFile)
	}

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return err
		}
	}

	return nil
}

// publicKeyFromBase58 converts a base58 encoded public key to a PublicKey struct
func publicKeyFromBase58(pub string) utils.PublicKey {
	var pubKey utils.PublicKey
	pubKey.KeyType = utils.ED25519
	pub = strings.TrimPrefix(pub, prefix)
	decoded := base58.Decode(pub)
	copy(pubKey.Data[:], decoded)
	return pubKey
}

// newKey generates a new key pair and returns the key file
func newKey(accountid string) *KeyFile {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}
	if accountid == "" {
		accountid = hex.EncodeToString(pub)
	}
	kf := &KeyFile{
		AccountID: accountid,
		PublicKey: prefix + base58.Encode(pub),
		SecretKey: prefix + base58.Encode(priv),
	}
	return kf
}

// dumpJson dumps the key file to a json file
// if the file already exists, it returns an error
func dumpJson(fileName string, keyFile *KeyFile) error {
	if _, err := os.Stat(fileName); os.IsNotExist(err) {
		file, err := os.Create(fileName)
		if err != nil {
			return err
		}
		defer file.Close()

		encoder := json.NewEncoder(file)
		encoder.SetIndent("", "  ")
		err = encoder.Encode(keyFile)
		if err != nil {
			return err
		}
		return nil
	} else {
		return fmt.Errorf("file already exists: [%s]", fileName)
	}
}

// loadConfig loads the configuration from the config file
func loadConfig() Config {
	var config Config
	err := viper.UnmarshalKey("endpoint.engine", &config)
	if err != nil {
		panic(err)
	}
	if config.NearNodeURL == "" {
		config.NearNodeURL = config.NearArchivalNodeURL
	}

	config.NearConfig = near.Config{
		NetworkID: config.NearNetworkID,
		NodeURL:   config.NearNodeURL,
		KeyPath:   config.SignerKey,
	}
	return config
}

// getFunctionCallKeyPairFilePaths returns the file paths of the key pairs that match the pattern
func getFunctionCallKeyPairFilePaths(path, prefixPattern string) []string {
	dir, file := filepath.Split(path)
	pattern := filepath.Join(dir, prefixPattern+file)

	keyPairFiles := make([]string, 0)
	files, err := filepath.Glob(pattern)
	if err == nil && len(files) > 0 {
		keyPairFiles = append(keyPairFiles, files...)
	}
	return keyPairFiles
}

func acceptConfigFilePath(cmd *cobra.Command) {
	cmd.PersistentFlags().StringP("config", "c", "config/testnet.yaml", "Path of the configuration file")
}
