package rollup

import (
	"math/big"
)

type Rollup struct {
	rpcURL  string
	chainID *big.Int
	name    string
}

func New(rpcURL string, chainID *big.Int, name string) *Rollup {
	return &Rollup{
		rpcURL:  rpcURL,
		chainID: chainID,
		name:    name,
	}
}
func (r *Rollup) RPCURL() string {
	return r.rpcURL
}

func (r *Rollup) ChainID() *big.Int {
	return r.chainID
}

func (r *Rollup) Name() string {
	return r.name
}
