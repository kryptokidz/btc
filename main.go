package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
)

// API constants.
const (
	APIEndpoint = "https://api.coinbase.com"
	APIVersion  = "2016-05-16"
)

func main() {
	var sinceDate string
	flag.StringVar(&sinceDate, "since", "", "ISO-8601 date")
	flag.Parse()
	since := time.Now().Add(-4 * 7 * 24 * time.Hour) // last 4 weeks
	if sinceDate != "" {
		s, err := time.Parse("2006-01-02", sinceDate)
		if err != nil {
			log.Fatal(err)
		}
		since = s
	}

	gains := calcGains(since)

	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 0, 0, 1, ' ', tabwriter.AlignRight)
	headers := []string{"", "Cost Basis", "Amount", "Value", "$", "%"}
	printLine(w, headers)
	printSep(w, headers)

	var sums struct {
		costBasis, nativeValue float64
	}
	for _, gain := range gains {
		sums.costBasis += gain.CostBasis
		sums.nativeValue += gain.NativeValue
		printLine(w, []string{
			gain.Currency,
			fmtUSD(gain.CostBasis),
			fmtVal(gain.Value),
			fmtVal(gain.NativeValue),
			sign(gain.Profit()) + fmtUSD(math.Abs(gain.Profit())),
			sign(gain.Profit()) + fmtPCT(math.Abs(gain.ProfitPercent())),
		})
	}
	profit := sums.nativeValue - sums.costBasis
	profitPCT := profit / sums.costBasis
	printSep(w, headers)
	printLine(w, []string{
		"Total",
		fmtUSD(sums.costBasis), // cost basis
		"", // value (crypto)
		fmtUSD(sums.nativeValue), // total usd value
		sign(profit) + fmtUSD(math.Abs(profit)),
		sign(profit) + fmtPCT(math.Abs(profitPCT)),
	})
	_ = w.Flush()
}

func sign(val float64) string {
	if val < 0 {
		return "-"
	}
	return "+"
}

func fmtPCT(val float64) string {
	return fmt.Sprintf("%.2f%%", val*100)
}

func fmtUSD(val float64) string {
	return fmt.Sprintf("$%.2f", val)
}

func fmtVal(val float64) string {
	return fmt.Sprintf("%.2f", val)
}

type Gains struct {
	Currency    string
	Value       float64
	CostBasis   float64
	NativeValue float64
}

func (g *Gains) Profit() float64 {
	return g.NativeValue - g.CostBasis
}

func (g *Gains) ProfitPercent() float64 {
	return g.Profit() / g.CostBasis
}

func calcGains(since time.Time) []*Gains {
	cb := &Coinbase{
		Key:    os.Getenv("COINBASE_KEY"),
		Secret: os.Getenv("COINBASE_SECRET"),
	}
	accounts, err := cb.GetAccounts()
	must(err)
	accountIDs := make([]string, 0)
	for _, a := range accounts {
		accountIDs = append(accountIDs, a.ID)
	}
	trans, err := cb.GetAllTransactions(accountIDs)
	must(err)

	amount := make(map[string]float64, 0)
	costBasis := make(map[string]float64, 0)

	for _, t := range trans {
		if t.CreatedAt.Before(since) {
			continue
		}
		switch t.Type {
		case "send":
			amount[t.Amount.Currency] += t.Amount.Amount()
		case "buy":
			amount[t.Amount.Currency] += t.Amount.Amount()
			costBasis[t.Amount.Currency] += t.Buy.Total.Amount()
		case "sell":
			amount[t.Amount.Currency] += t.Amount.Amount()
			costBasis[t.Amount.Currency] -= t.Sell.Total.Amount()
		default:
			panic("Unhandled transaction type: " + t.Type)
		}
	}
	spot, err := cb.GetSpotRates()
	must(err)
	gains := make([]*Gains, 0)
	for _, s := range spot {
		val := amount[s.Base]
		if val == 0 {
			continue
		}
		gains = append(gains, &Gains{
			Currency:    s.Base,
			Value:       val,
			CostBasis:   costBasis[s.Base],
			NativeValue: val * s.Amount(),
		})
	}

	return gains
}

func js(v interface{}) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

type Amount struct {
	RawAmount string `json:"amount"`
	Currency  string `json:"currency"`
}

func (a Amount) Amount() float64 {
	f, err := strconv.ParseFloat(a.RawAmount, 64)
	if err != nil {
		panic(err)
	}
	return f
}

type Transaction struct {
	Type         string    `json:"type"`
	Amount       Amount    `json:"amount"`
	NativeAmount Amount    `json:"native_amount"`
	CreatedAt    time.Time `json:"created_at"`
	Buy          struct {
		Fee      Amount `json:"fee"`
		Amount   Amount `json:"amount"`
		Total    Amount `json:"total"`
		Subtotal Amount `json:"subtotal"`
	} `json:"buy"`
	Sell struct {
		Fee      Amount `json:"fee"`
		Amount   Amount `json:"amount"`
		Total    Amount `json:"total"`
		Subtotal Amount `json:"subtotal"`
	} `json:"sell"`
}

type Coinbase struct {
	Key    string
	Secret string
}

func (c *Coinbase) Authenticate(path string, req *http.Request) error {
	timestamp := strconv.FormatInt(time.Now().UTC().Unix(), 10)
	message := timestamp + req.Method + path

	sha := sha256.New
	h := hmac.New(sha, []byte(c.Secret))
	h.Write([]byte(message))

	signature := fmt.Sprintf("%x", h.Sum(nil))

	req.Header.Set("CB-ACCESS-KEY", c.Key)
	req.Header.Set("CB-ACCESS-SIGN", signature)
	req.Header.Set("CB-ACCESS-TIMESTAMP", timestamp)

	return nil
}

func (c *Coinbase) get(path string, value interface{}) error {
	req, err := http.NewRequest(http.MethodGet, APIEndpoint+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("CB-Version", APIVersion)
	if !strings.HasSuffix(path, "time") {
		err = c.Authenticate(path, req)
		if err != nil {
			return err
		}
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return errors.New(resp.Status + ": " + path)
	}
	if err := json.NewDecoder(resp.Body).Decode(value); err != nil {
		return err
	}
	return nil
}

func (c *Coinbase) GetTransactions(account string) ([]*Transaction, error) {
	url := "/v2/accounts/" + account + "/transactions?limit=100&expand=all"
	var result struct {
		Data []*Transaction `json:"data"`
	}
	if err := c.get(url, &result); err != nil {
		return nil, err
	}
	return result.Data, nil
}

func (c *Coinbase) GetAllTransactions(accounts []string) ([]*Transaction, error) {
	result := make([]*Transaction, 0)
	for _, account := range accounts {
		t, err := c.GetTransactions(account)
		if err != nil {
			return nil, err
		}
		result = append(result, t...)
	}
	return result, nil
}

type Account struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (c *Coinbase) GetAccounts() ([]*Account, error) {
	url := "/v2/accounts"
	var result struct {
		Data []*Account `json:"data"`
	}
	if err := c.get(url, &result); err != nil {
		return nil, err
	}
	return result.Data, nil
}

type SpotRate struct {
	Base      string `json:"base"`
	Currency  string `json:"currency"`
	RawAmount string `json:"amount"`
}

func (s SpotRate) Amount() float64 {
	f, err := strconv.ParseFloat(s.RawAmount, 64)
	if err != nil {
		panic(err)
	}
	return f
}

func (c *Coinbase) GetSpotRates() ([]*SpotRate, error) {
	url := "/v2/prices/USD/spot"
	var result struct {
		Data []*SpotRate `json:"data"`
	}
	if err := c.get(url, &result); err != nil {
		return nil, err
	}
	return result.Data, nil
}

func maxInt(x, y int) int {
	if x > y {
		return x
	}
	return y
}

func printLine(w io.Writer, v []string) {
	fmt.Fprintln(w, strings.Join(v, "\t")+"\t")
}

func printSep(w io.Writer, v []string) {
	vv := make([]string, 0, len(v))
	for _, col := range v {
		vv = append(vv, strings.Repeat("-", maxInt(len(col), 8)))
	}
	printLine(w, vv)
}
