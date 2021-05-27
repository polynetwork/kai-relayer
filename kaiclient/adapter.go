package kaiclient

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"

	kai "github.com/kardiachain/go-kardia/mainchain"
)

func (ec *Client) toEthReceipt(r *kai.PublicReceipt) *types.Receipt {
	return &types.Receipt{
		Status:            uint64(r.Status),
		CumulativeGasUsed: r.CumulativeGasUsed,
		Bloom:             types.BytesToBloom(r.LogsBloom.Bytes()),
		Logs:              ec.toEthLogs(r.Logs),
		TxHash:            common.HexToHash(r.TransactionHash),
		ContractAddress:   common.HexToAddress(r.ContractAddress),
		GasUsed:           r.GasUsed,
		BlockHash:         common.HexToHash(r.BlockHash),
		BlockNumber:       new(big.Int).SetUint64(r.BlockHeight),
		TransactionIndex:  uint(r.TransactionIndex),
	}
}

func (ec *Client) toEthLogs(kaiLogs []kai.Log) []*types.Log {
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
