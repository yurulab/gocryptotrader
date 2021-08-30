package simulator

import (
	"testing"

	"github.com/yurulab/gocryptotrader/currency"
	"github.com/yurulab/gocryptotrader/exchanges/asset"
	"github.com/yurulab/gocryptotrader/exchanges/bitstamp"
)

func TestSimulate(t *testing.T) {
	b := bitstamp.Bitstamp{}
	b.SetDefaults()
	b.Verbose = false
	o, err := b.FetchOrderbook(currency.NewPair(currency.BTC, currency.USD), asset.Spot)
	if err != nil {
		t.Error(err)
	}

	r := o.SimulateOrder(10000000, true)
	t.Log(r.Status)
	r = o.SimulateOrder(2171, false)
	t.Log(r.Status)
}
