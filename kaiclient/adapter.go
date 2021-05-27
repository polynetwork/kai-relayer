package kaiclient

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"

	kai "github.com/kardiachain/go-kardia/mainchain"
)

func (ec *Client) toEthReceipt(raw *kai.PublicReceipt) *types.Receipt {
	return &types.Receipt{
		Status:            uint64(raw.Status),
		CumulativeGasUsed: raw.CumulativeGasUsed,
		Bloom:             types.BytesToBloom(raw.LogsBloom.Bytes()),
		Logs:              toEthLogs(raw.Logs),
		TxHash:            common.HexToHash(raw.TransactionHash),
		ContractAddress:   common.HexToAddress(raw.ContractAddress),
		GasUsed:           raw.GasUsed,
		BlockHash:         common.HexToHash(raw.BlockHash),
		BlockNumber:       new(big.Int).SetUint64(raw.BlockHeight),
		TransactionIndex:  uint(raw.TransactionIndex),
	}
}

func toEthLogs(kaiLogs []kai.Log) []*types.Log {
	logs := make([]*types.Log, len(kaiLogs))
	for i := range kaiLogs {
		topics := make([]common.Hash, len(kaiLogs[i].Topics))
		for j := range kaiLogs[i].Topics {
			topics[j] = common.HexToHash(kaiLogs[i].Topics[j])
		}
		logs[i] = &types.Log{
			Address:     common.HexToAddress(kaiLogs[i].Address),
			Topics:      topics,
			Data:        common.Hex2Bytes(kaiLogs[i].Data),
			BlockNumber: kaiLogs[i].BlockHeight,
			TxHash:      common.HexToHash(kaiLogs[i].TxHash),
			TxIndex:     kaiLogs[i].TxIndex,
			BlockHash:   common.HexToHash(kaiLogs[i].BlockHash),
			Index:       kaiLogs[i].Index,
			Removed:     kaiLogs[i].Removed,
		}
	}
	return logs
}

type rpcTransaction struct {
	tx *types.Transaction
	txExtraInfo
}

type txExtraInfo struct {
	BlockNumber *string         `json:"blockNumber,omitempty"`
	BlockHash   *common.Hash    `json:"blockHash,omitempty"`
	From        *common.Address `json:"from,omitempty"`
}

// RPCTransaction represents a transaction that will serialize to the RPC representation of a transaction
type RPCTransaction struct {
	BlockHash        *common.Hash    `json:"blockHash"`
	BlockNumber      *hexutil.Big    `json:"blockNumber"`
	From             common.Address  `json:"from"`
	Gas              hexutil.Uint64  `json:"gas"`
	GasPrice         *hexutil.Big    `json:"gasPrice"`
	Hash             common.Hash     `json:"hash"`
	Input            hexutil.Bytes   `json:"input"`
	Nonce            hexutil.Uint64  `json:"nonce"`
	To               *common.Address `json:"to"`
	TransactionIndex *hexutil.Uint64 `json:"transactionIndex"`
	Value            *hexutil.Big    `json:"value"`
	ChainID          *hexutil.Big    `json:"chainId,omitempty"`
	V                *hexutil.Big    `json:"v"`
	R                *hexutil.Big    `json:"r"`
	S                *hexutil.Big    `json:"s"`
}

func toEthRPCTransaction(raw *kai.PublicTransaction) *RPCTransaction {
	blockHash := common.HexToHash(raw.BlockHash)
	to := common.HexToAddress(raw.To)
	transactionIndex := hexutil.Uint64(raw.TransactionIndex)
	value, _ := new(big.Int).SetString(raw.Value, 10)
	return &RPCTransaction{
		BlockHash:        &blockHash,
		BlockNumber:      (*hexutil.Big)(new(big.Int).SetUint64(raw.BlockHeight)),
		From:             common.HexToAddress(raw.From),
		Gas:              hexutil.Uint64(raw.Gas),
		GasPrice:         (*hexutil.Big)(new(big.Int).SetUint64(raw.GasPrice)),
		Hash:             common.HexToHash(raw.Hash),
		Input:            common.Hex2Bytes(raw.Input),
		Nonce:            hexutil.Uint64(raw.Nonce),
		To:               &to,
		TransactionIndex: &transactionIndex,
		Value:            (*hexutil.Big)(value),
		V:                (*hexutil.Big)(raw.V),
		R:                (*hexutil.Big)(raw.R),
		S:                (*hexutil.Big)(raw.S),
	}
}

//// UnmarshalJSON decodes the web3 RPC transaction format.
//func (tx *Transaction) UnmarshalJSON(input []byte) error {
//	var dec txdata
//	if err := dec.UnmarshalJSON(input); err != nil {
//		return err
//	}
//
//	withSignature := dec.V.Sign() != 0 || dec.R.Sign() != 0 || dec.S.Sign() != 0
//	if withSignature {
//		var V byte
//		if isProtectedV(dec.V) {
//			chainID := deriveChainId(dec.V).Uint64()
//			V = byte(dec.V.Uint64() - 35 - 2*chainID)
//		} else {
//			V = byte(dec.V.Uint64() - 27)
//		}
//		if !crypto.ValidateSignatureValues(V, dec.R, dec.S, false) {
//			return ErrInvalidSig
//		}
//	}
//
//	*tx = Transaction{data: dec}
//	return nil
//}
//
//// UnmarshalJSON unmarshals from JSON.
//func (t *txdata) UnmarshalJSON(input []byte) error {
//	type txdata struct {
//		AccountNonce *hexutil.Uint64 `json:"nonce"    gencodec:"required"`
//		Price        *hexutil.Big    `json:"gasPrice" gencodec:"required"`
//		GasLimit     *hexutil.Uint64 `json:"gas"      gencodec:"required"`
//		Recipient    *common.Address `json:"to"       rlp:"nil"`
//		Amount       *hexutil.Big    `json:"value"    gencodec:"required"`
//		Payload      *hexutil.Bytes  `json:"input"    gencodec:"required"`
//		V            *hexutil.Big    `json:"v" gencodec:"required"`
//		R            *hexutil.Big    `json:"r" gencodec:"required"`
//		S            *hexutil.Big    `json:"s" gencodec:"required"`
//		Hash         *common.Hash    `json:"hash" rlp:"-"`
//	}
//	var dec txdata
//	if err := json.Unmarshal(input, &dec); err != nil {
//		return err
//	}
//	if dec.AccountNonce == nil {
//		return errors.New("missing required field 'nonce' for txdata")
//	}
//	t.AccountNonce = uint64(*dec.AccountNonce)
//	if dec.Price == nil {
//		return errors.New("missing required field 'gasPrice' for txdata")
//	}
//	t.Price = (*big.Int)(dec.Price)
//	if dec.GasLimit == nil {
//		return errors.New("missing required field 'gas' for txdata")
//	}
//	t.GasLimit = uint64(*dec.GasLimit)
//	if dec.Recipient != nil {
//		t.Recipient = dec.Recipient
//	}
//	if dec.Amount == nil {
//		return errors.New("missing required field 'value' for txdata")
//	}
//	t.Amount = (*big.Int)(dec.Amount)
//	if dec.Payload == nil {
//		return errors.New("missing required field 'input' for txdata")
//	}
//	t.Payload = *dec.Payload
//	if dec.V == nil {
//		return errors.New("missing required field 'v' for txdata")
//	}
//	t.V = (*big.Int)(dec.V)
//	if dec.R == nil {
//		return errors.New("missing required field 'r' for txdata")
//	}
//	t.R = (*big.Int)(dec.R)
//	if dec.S == nil {
//		return errors.New("missing required field 's' for txdata")
//	}
//	t.S = (*big.Int)(dec.S)
//	if dec.Hash != nil {
//		t.Hash = dec.Hash
//	}
//	return nil
//}
