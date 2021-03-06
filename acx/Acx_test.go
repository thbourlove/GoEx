package acx

import (
	"github.com/thbourlove/GoEx"
	"net/http"
	"testing"
)

var acx = New(http.DefaultClient, "", "")

func TestAcx_GetTicker(t *testing.T) {
	AUD := goex.NewCurrency("AUD", "")
	BTC_AUD := goex.NewCurrencyPair(goex.BTC, AUD)
	ticker, err := acx.GetTicker(BTC_AUD)
	t.Log("err=>", err)
	t.Log("ticker=>", ticker)
}
