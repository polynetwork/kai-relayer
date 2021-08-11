// Copyright 2016 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

// Package kaiclient provides a client for the KardiaChain RPC API.
package kaiclient

import (
	"context"
	"math"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"

	kai "github.com/kardiachain/go-kardia/mainchain"
	ktypes "github.com/kardiachain/go-kardia/types"
)

// Client defines typed wrappers for the Ethereum RPC API.
type Client struct {
	c *rpc.Client
}

// Dial connects a client to the given URL.
func Dial(rawurl string) (*Client, error) {
	return DialContext(context.Background(), rawurl)
}

func DialContext(ctx context.Context, rawurl string) (*Client, error) {
	c, err := rpc.DialContext(ctx, rawurl)
	if err != nil {
		return nil, err
	}
	return NewClient(c), nil
}

// NewClient creates a client that uses the given RPC client.
func NewClient(c *rpc.Client) *Client {
	return &Client{c}
}

func (ec *Client) Close() {
	ec.c.Close()
}

// HeaderByNumber returns a block header from the current canonical chain. If number is
// nil, the latest known header is returned.
func (ec *Client) HeaderByNumber(ctx context.Context, number *big.Int) (*ktypes.Header, error) {
	var head *ktypes.Header
	err := ec.c.CallContext(ctx, &head, "kai_getBlockHeaderByNumber", toBlockNumArg(number))
	if err == nil && head == nil {
		err = ethereum.NotFound
	}
	return head, err
}

type KaiHeader struct {
	Header       *ktypes.Header
	Commit       *ktypes.Commit
	ValidatorSet *ktypes.ValidatorSet
}

func (ec *Client) FullHeaderByNumber(ctx context.Context, number *big.Int) (*KaiHeader, error) {
	header, err := ec.HeaderByNumber(ctx, number)
	if err != nil {
		return nil, err
	}
	validators, err := ec.GetValidators(ctx, number)
	if err != nil {
		return nil, err
	}

	commit, err := ec.GetCommit(ctx, number.Sub(number, big.NewInt(1)))
	if err != nil {
		return nil, err
	}
	return &KaiHeader{
		Header:       header,
		ValidatorSet: validators,
		Commit:       commit,
	}, nil
}

func (ec *Client) GetValidators(ctx context.Context, number *big.Int) (*ktypes.ValidatorSet, error) {
	var valSet *ktypes.ValidatorSet
	err := ec.c.CallContext(ctx, &valSet, "kai_getValidatorSet", toBlockNumArg(number))
	if err == nil && valSet == nil {
		err = ethereum.NotFound
	}
	return valSet, err
}

func (ec *Client) GetCommit(ctx context.Context, number *big.Int) (*ktypes.Commit, error) {
	var commit *ktypes.Commit
	err := ec.c.CallContext(ctx, &commit, "kai_getCommit", toBlockNumArg(number))
	if err == nil && commit == nil {
		err = ethereum.NotFound
	}
	return commit, err
}

func (ec *Client) GetProof(ctx context.Context, address common.Address, storageKeys []string, number *big.Int) (*kai.AccountResult, error) {
	var accountR *kai.AccountResult
	err := ec.c.CallContext(ctx, &accountR, "kai_getProof", address, storageKeys, toBlockNumArg(number), false)
	if err == nil && accountR == nil {
		err = ethereum.NotFound
	}
	return accountR, err
}
func toBlockNumArg(number *big.Int) string {
	if number == nil {
		return "latest"
	}
	pending := new(big.Int).SetUint64(math.MaxUint64 - 1)
	if number.Cmp(pending) == 0 {
		return "pending"
	}
	return number.String()
}
