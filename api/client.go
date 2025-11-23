package api

import "github.com/cry999/atcoder-cli/contests"

type Client struct {
	family contests.Family
}

func NewClient(family contests.Family) *Client {
	return &Client{
		family: family,
	}
}
