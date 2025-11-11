package configs

import (
	_ "embed"
	"errors"
	"fmt"

	"github.com/compose-network/dome/internal/logger"
	"github.com/ethereum/go-ethereum/common"
	"gopkg.in/yaml.v3"
)

var (
	//go:embed config.yaml
	config []byte
	Values App
)

const (
	ChainNameRollupA ChainName = "rollup-a"
	ChainNameRollupB ChainName = "rollup-b"

	ContractNameBridge   ContractName = "bridge"
	ContractNamePingPong ContractName = "pingpong"
	ContractNameToken    ContractName = "bridgeabletoken"
)

type (
	ChainName    string
	ContractName string

	App struct {
		L2 L2 `yaml:"l2"`
	}
	L2 struct {
		ChainConfigs map[ChainName]ChainConfig       `yaml:"chain-configs"`
		Contracts    map[ContractName]ContractConfig `yaml:"contracts"`
	}
	ChainConfig struct {
		ID     int64  `yaml:"id"`
		RPCURL string `yaml:"rpc-url"`
		PK     string `yaml:"pk"`
	}

	ContractConfig struct {
		Address common.Address `yaml:"address"`
		ABI     string         `yaml:"abi"`
	}
)

func (a *App) validate() error {
	var err error

	if chainErr := a.validateChainConfig(); chainErr != nil {
		err = errors.Join(err, chainErr)
	}

	if contractsErr := a.validateContractsConfig(); contractsErr != nil {
		err = errors.Join(err, contractsErr)
	}

	return err
}

func (a *App) validateChainConfig() error {
	var err error
	if len(a.L2.ChainConfigs) != 2 {
		err = errors.Join(err, fmt.Errorf("exactly two chain configs must be provided"))
	}
	if _, ok := a.L2.ChainConfigs[ChainNameRollupA]; !ok {
		err = errors.Join(err, fmt.Errorf("chain config for '%s' must be provided", ChainNameRollupA))
	}
	if _, ok := a.L2.ChainConfigs[ChainNameRollupB]; !ok {
		err = errors.Join(err, fmt.Errorf("chain config for '%s' must be provided", ChainNameRollupB))
	}

	for name, cfg := range a.L2.ChainConfigs {
		if cfg.ID == 0 {
			err = errors.Join(err, fmt.Errorf("field: 'id', chain: '%s', must be set and non-zero", name))
		}
		if cfg.RPCURL == "" {
			err = errors.Join(err, fmt.Errorf("field: 'rpc-url', chain: '%s', must be set and non-zero", name))
		}
		if cfg.PK == "" {
			err = errors.Join(err, fmt.Errorf("field: 'pk', chain: '%s', must be set and non-zero", name))
		}
	}

	return err
}

func (a *App) validateContractsConfig() error {
	var err error
	if len(a.L2.Contracts) != 3 {
		err = errors.Join(err, fmt.Errorf("exactly three contract configs must be provided"))
	}
	if _, ok := a.L2.Contracts[ContractNameBridge]; !ok {
		err = errors.Join(err, fmt.Errorf("contract config for '%s' must be provided", ContractNameBridge))
	}
	if _, ok := a.L2.Contracts[ContractNamePingPong]; !ok {
		err = errors.Join(err, fmt.Errorf("contract config for '%s' must be provided", ContractNamePingPong))
	}
	if _, ok := a.L2.Contracts[ContractNameToken]; !ok {
		err = errors.Join(err, fmt.Errorf("contract config for '%s' must be provided", ContractNameToken))
	}

	for name, cfg := range a.L2.Contracts {
		if cfg.Address == (common.Address{}) {
			err = errors.Join(err, fmt.Errorf("field: 'address', contract: '%s', must be set and non-zero", name))
		}
		if cfg.ABI == "" {
			err = errors.Join(err, fmt.Errorf("field: 'abi', contract: '%s', must be set and non-empty", name))
		}
	}

	return err
}

func init() {
	if err := yaml.Unmarshal(config, &Values); err != nil {
		panic("Failed to unmarshal config: " + err.Error())
	}

	if err := Values.validate(); err != nil {
		panic("invalid config: " + err.Error())
	}

	logger.Info("configuration was loaded successfully: %v", Values)
}
