package gct

import "github.com/yurulab/gocryptotrader/gctscript/wrappers/gct/exchange"

// Setup returns a Wrapper
func Setup() *Wrapper {
	return &Wrapper{
		&exchange.Exchange{},
	}
}
