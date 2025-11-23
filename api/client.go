package api

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/cry999/atcoder-cli/contests"
)

type Client struct {
	family   contests.Family
	interval time.Duration

	reqCh        chan *request
	shutdownOnce sync.Once
	shutdownCh   chan struct{}
}

type request struct {
	req  *http.Request
	resp chan *http.Response
	err  chan error
}

func NewClient(family contests.Family) *Client {
	c := &Client{
		family:   family,
		interval: 100 * time.Millisecond,

		reqCh:        make(chan *request),
		shutdownOnce: sync.Once{},
		shutdownCh:   make(chan struct{}),
	}

	go c.requestLoop()

	return c
}

func (c *Client) Shutdown() {
	c.shutdownOnce.Do(func() { close(c.shutdownCh); close(c.reqCh) })
}

func (c *Client) do(req *http.Request) (*http.Response, error) {
	respCh := make(chan *http.Response)
	errCh := make(chan error)
	c.reqCh <- &request{
		req:  req,
		resp: respCh,
		err:  errCh,
	}
	select {
	case resp := <-respCh:
		return resp, nil
	case err := <-errCh:
		return nil, err
	case <-c.shutdownCh:
		return nil, fmt.Errorf("client is shut down")
	}
}

func (c *Client) requestLoop() {
	for {
		select {
		case req := <-c.reqCh:
			resp, err := http.DefaultClient.Do(req.req)
			if err != nil {
				req.err <- err
			} else {
				req.resp <- resp
			}
			close(req.err)
			close(req.resp)
		case <-c.shutdownCh:
			return
		}
		select {
		case <-time.After(c.interval):
		case <-c.shutdownCh:
			return
		}
	}
}
