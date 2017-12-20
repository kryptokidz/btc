package coinbase

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// API constants.
const (
	APIEndpoint = "https://api.coinbase.com"
	APIVersion  = "2016-05-16"
)

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

type Client struct {
	Key    string
	Secret string
}

func (c *Client) Authenticate(path string, req *http.Request) error {
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

func (c *Client) get(path string, value interface{}) error {
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

func (c *Client) GetTransactions(account string) ([]*Transaction, error) {
	url := "/v2/accounts/" + account + "/transactions?limit=100&expand=all"
	var result struct {
		Data []*Transaction `json:"data"`
	}
	if err := c.get(url, &result); err != nil {
		return nil, err
	}
	return result.Data, nil
}

func (c *Client) GetAllTransactions(accounts []string) ([]*Transaction, error) {
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

func (c *Client) GetAccounts() ([]*Account, error) {
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

func (c *Client) GetSpotRates() ([]*SpotRate, error) {
	url := "/v2/prices/USD/spot"
	var result struct {
		Data []*SpotRate `json:"data"`
	}
	if err := c.get(url, &result); err != nil {
		return nil, err
	}
	return result.Data, nil
}
