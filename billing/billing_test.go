package billing

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chimixiong/deepseek-sdk-go"
)

func TestGetBalance(t *testing.T) {
	const fixture = `{
  "is_available": true,
  "balance_infos": [
    {
      "currency": "CNY",
      "total_balance": "110.00",
      "granted_balance": "10.00",
      "topped_up_balance": "100.00"
    },
    {
      "currency": "USD",
      "total_balance": "0.50",
      "granted_balance": "0.00",
      "topped_up_balance": "0.50"
    }
  ]
}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/user/balance" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, fixture)
	}))
	defer server.Close()

	client := New(deepseek.New(deepseek.Config{APIKey: "test-key", BaseURL: server.URL}))
	balance, err := client.GetBalance(context.Background())
	if err != nil {
		t.Fatalf("GetBalance: %v", err)
	}
	if !balance.IsAvailable {
		t.Error("IsAvailable = false")
	}
	if len(balance.BalanceInfos) != 2 {
		t.Fatalf("balance infos = %d", len(balance.BalanceInfos))
	}
	cny := balance.BalanceInfos[0]
	if cny.Currency != CurrencyCNY || cny.TotalBalance != "110.00" ||
		cny.GrantedBalance != "10.00" || cny.ToppedUpBalance != "100.00" {
		t.Errorf("CNY balance = %#v", cny)
	}
	usd := balance.BalanceInfos[1]
	if usd.Currency != CurrencyUSD || usd.TotalBalance != "0.50" {
		t.Errorf("USD balance = %#v", usd)
	}
}

func TestNewNilClient(t *testing.T) {
	if client := New(nil); client == nil || client.api == nil {
		t.Fatal("New(nil) did not install a default transport client")
	}
}
