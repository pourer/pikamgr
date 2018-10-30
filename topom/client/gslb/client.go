package gslb

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
	"context"
)

type Client struct {
	addr    string
	timeout time.Duration
}

func NewClient(addr string, timeout time.Duration) *Client {
	return &Client{
		addr:    addr,
		timeout: timeout,
	}
}

func (c *Client) Info() ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://%s/status", c.addr), nil)
	if err != nil {
		return nil, err
	}
	ctx, _ := context.WithTimeout(context.Background(), c.timeout)
	req.WithContext(ctx)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http response status code not ok. statusCode:%d", resp.StatusCode)
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return data, nil
}
