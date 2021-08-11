/*
* Copyright (C) 2020 The poly network Authors
* This file is part of The poly network library.
*
* The poly network is free software: you can redistribute it and/or modify
* it under the terms of the GNU Lesser General Public License as published by
* the Free Software Foundation, either version 3 of the License, or
* (at your option) any later version.
*
* The poly network is distributed in the hope that it will be useful,
* but WITHOUT ANY WARRANTY; without even the implied warranty of
* MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
* GNU Lesser General Public License for more details.
* You should have received a copy of the GNU Lesser General Public License
* along with The poly network . If not, see <http://www.gnu.org/licenses/>.
 */
package tools

import (
	"bytes"
	"crypto/ed25519"
	"crypto/elliptic"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	"github.com/btcsuite/btcd/btcec"
	"github.com/ontio/ontology-crypto/ec"
	"github.com/ontio/ontology-crypto/keypair"
	"github.com/ontio/ontology-crypto/sm2"
	"github.com/polynetwork/poly/common"

	ktypes "github.com/kardiachain/go-kardia/types"
)

type jsonError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type heightReq struct {
	JsonRpc string   `json:"jsonrpc"`
	Method  string   `json:"method"`
	Params  []string `json:"params"`
	Id      uint     `json:"id"`
}

type heightRsp struct {
	JsonRpc string     `json:"jsonrpc"`
	Result  uint64     `json:"result,omitempty"`
	Error   *jsonError `json:"error,omitempty"`
	Id      uint       `json:"id"`
}

type proofReq struct {
	JsonRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	Id      uint          `json:"id"`
}

type proofRsp struct {
	JsonRPC string     `json:"jsonrpc"`
	Result  KAIProof   `json:"result,omitempty"`
	Error   *jsonError `json:"error,omitempty"`
	Id      uint       `json:"id"`
}

type KAIProof struct {
	Address       string         `json:"address"`
	Balance       string         `json:"balance"`
	CodeHash      string         `json:"codeHash"`
	Nonce         string         `json:"nonce"`
	StorageHash   string         `json:"storageHash"`
	AccountProof  []string       `json:"accountProof"`
	StorageProofs []StorageProof `json:"storageProof"`
}

type StorageProof struct {
	Key   string   `json:"key"`
	Value string   `json:"value"`
	Proof []string `json:"proof"`
}

type blockReq struct {
	JsonRpc string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	Id      uint          `json:"id"`
}

type blockRsp struct {
	JsonRPC string         `json:"jsonrpc"`
	Result  *ktypes.Header `json:"result,omitempty"`
	Error   *jsonError     `json:"error,omitempty"`
	Id      uint           `json:"id"`
}

type valSetRsp struct {
	JsonRPC string              `json:"jsonrpc"`
	Result  []*ktypes.Validator `json:"result,omitempty"`
	Error   *jsonError          `json:"error,omitempty"`
	Id      uint                `json:"id"`
}

type commitRsp struct {
	JsonRPC string         `json:"jsonrpc"`
	Result  *ktypes.Commit `json:"result,omitempty"`
	Error   *jsonError     `json:"error,omitempty"`
	Id      uint           `json:"id"`
}

func GetNodeHeader(url string, restClient *RestClient, height uint64) ([]byte, error) {
	params := []interface{}{fmt.Sprintf("0x%x", height), true}
	req := &blockReq{
		JsonRpc: "2.0",
		Method:  "kai_getBlockByNumber",
		Params:  params,
		Id:      1,
	}
	reqdata, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("GetNodeHeight: marshal req err: %s", err)
	}
	rspdata, err := restClient.SendRestRequest(url, reqdata)
	if err != nil {
		return nil, fmt.Errorf("GetNodeHeight err: %s", err)
	}
	rsp := &blockRsp{}
	err = json.Unmarshal(rspdata, rsp)
	if err != nil {
		return nil, fmt.Errorf("GetNodeHeight, unmarshal resp err: %s", err)
	}
	if rsp.Error != nil {
		return nil, fmt.Errorf("GetNodeHeight, unmarshal resp err: %s", rsp.Error.Message)
	}
	return json.Marshal(rsp.Result)
}

func GetNodeHeight(url string, restClient *RestClient) (uint64, error) {
	req := &heightReq{
		JsonRpc: "2.0",
		Method:  "kai_blockNumber",
		Params:  make([]string, 0),
		Id:      1,
	}
	reqData, err := json.Marshal(req)
	if err != nil {
		return 0, fmt.Errorf("GetNodeHeight: marshal req err: %s", err)
	}
	rspData, err := restClient.SendRestRequest(url, reqData)
	if err != nil {
		return 0, fmt.Errorf("GetNodeHeight err: %s", err)
	}
	rsp := &heightRsp{}
	err = json.Unmarshal(rspData, rsp)
	if err != nil {
		return 0, fmt.Errorf("GetNodeHeight, unmarshal resp err: %s", err)
	}
	if rsp.Error != nil {
		return 0, fmt.Errorf("GetNodeHeight, unmarshal resp err: %s", rsp.Error.Message)
	}
	return rsp.Result, nil
}

func GetProof(url string, contractAddress string, key string, blockheight string, restClient *RestClient) ([]byte, error) {
	req := &proofReq{
		JsonRPC: "2.0",
		Method:  "eth_getProof",
		Params:  []interface{}{contractAddress, []string{key}, blockheight},
		Id:      1,
	}
	reqdata, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("eth_kaiProof: marshal req err: %s", err)
	}
	rspdata, err := restClient.SendRestRequest(url, reqdata)
	if err != nil {
		return nil, fmt.Errorf("GetProof: send request err: %s", err)
	}
	rsp := &proofRsp{}
	err = json.Unmarshal(rspdata, rsp)
	if err != nil {
		return nil, fmt.Errorf("GetProof, unmarshal resp err: %s", err)
	}
	if rsp.Error != nil {
		return nil, fmt.Errorf("GetNodeHeight, unmarshal resp err: %s", rsp.Error.Message)
	}
	result, err := json.Marshal(rsp.Result)
	if err != nil {
		return nil, fmt.Errorf("GetProof, Marshal result err: %s", err)
	}
	//fmt.Printf("proof res is:%s\n", string(result))
	return result, nil
}

func EncodeBigInt(b *big.Int) string {
	if b.Uint64() == 0 {
		return "00"
	}
	return hex.EncodeToString(b.Bytes())
}

func ParseAuditpath(path []byte) ([]byte, []byte, [][32]byte, error) {
	source := common.NewZeroCopySource(path)
	/*
		l, eof := source.NextUint64()
		if eof {
			return nil, nil, nil, nil
		}
	*/
	value, eof := source.NextVarBytes()
	if eof {
		return nil, nil, nil, nil
	}
	size := int((source.Size() - source.Pos()) / common.UINT256_SIZE)
	pos := make([]byte, 0)
	hashs := make([][32]byte, 0)
	for i := 0; i < size; i++ {
		f, eof := source.NextByte()
		if eof {
			return nil, nil, nil, nil
		}
		pos = append(pos, f)

		v, eof := source.NextHash()
		if eof {
			return nil, nil, nil, nil
		}
		var onehash [32]byte
		copy(onehash[:], (v.ToArray())[0:32])
		hashs = append(hashs, onehash)
	}

	return value, pos, hashs, nil
}

func GetNoCompresskey(key keypair.PublicKey) []byte {
	var buf bytes.Buffer
	switch t := key.(type) {
	case *ec.PublicKey:
		switch t.Algorithm {
		case ec.ECDSA:
			// Take P-256 as a special case
			if t.Params().Name == elliptic.P256().Params().Name {
				return ec.EncodePublicKey(t.PublicKey, false)
			}
			buf.WriteByte(byte(0x12))
		case ec.SM2:
			buf.WriteByte(byte(0x13))
		}
		label, err := GetCurveLabel(t.Curve.Params().Name)
		if err != nil {
			panic(err)
		}
		buf.WriteByte(label)
		buf.Write(ec.EncodePublicKey(t.PublicKey, false))
	case ed25519.PublicKey:
		panic("err")
	default:
		panic("err")
	}
	return buf.Bytes()
}

func GetCurveLabel(name string) (byte, error) {
	switch strings.ToUpper(name) {
	case strings.ToUpper(elliptic.P224().Params().Name):
		return 1, nil
	case strings.ToUpper(elliptic.P256().Params().Name):
		return 2, nil
	case strings.ToUpper(elliptic.P384().Params().Name):
		return 3, nil
	case strings.ToUpper(elliptic.P521().Params().Name):
		return 4, nil
	case strings.ToUpper(sm2.SM2P256V1().Params().Name):
		return 20, nil
	case strings.ToUpper(btcec.S256().Name):
		return 5, nil
	default:
		panic("err")
	}
}
