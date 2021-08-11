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
package manager

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/polynetwork/eth-contracts/go_abi/eccm_abi"
	"github.com/polynetwork/kai-relayer/config"
	"github.com/polynetwork/kai-relayer/db"
	"github.com/polynetwork/kai-relayer/kaiclient"

	"context"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/polynetwork/kai-relayer/log"
	"github.com/polynetwork/kai-relayer/tools"
	sdk "github.com/polynetwork/poly-go-sdk"
	"github.com/polynetwork/poly/common"
	"github.com/polynetwork/poly/native/service/cross_chain_manager/eth"
	scom "github.com/polynetwork/poly/native/service/header_sync/common"
	"github.com/polynetwork/poly/native/service/utils"
	autils "github.com/polynetwork/poly/native/service/utils"
)

type CrossTransfer struct {
	txIndex string
	txId    []byte
	value   []byte
	toChain uint32
	height  uint64
}

func (this *CrossTransfer) Serialization(sink *common.ZeroCopySink) {
	sink.WriteString(this.txIndex)
	sink.WriteVarBytes(this.txId)
	sink.WriteVarBytes(this.value)
	sink.WriteUint32(this.toChain)
	sink.WriteUint64(this.height)
}

func (this *CrossTransfer) Deserialization(source *common.ZeroCopySource) error {
	txIndex, eof := source.NextString()
	if eof {
		return fmt.Errorf("Waiting deserialize txIndex error")
	}
	txId, eof := source.NextVarBytes()
	if eof {
		return fmt.Errorf("Waiting deserialize txId error")
	}
	value, eof := source.NextVarBytes()
	if eof {
		return fmt.Errorf("Waiting deserialize value error")
	}
	toChain, eof := source.NextUint32()
	if eof {
		return fmt.Errorf("Waiting deserialize toChain error")
	}
	height, eof := source.NextUint64()
	if eof {
		return fmt.Errorf("Waiting deserialize height error")
	}
	this.txIndex = txIndex
	this.txId = txId
	this.value = value
	this.toChain = toChain
	this.height = height
	return nil
}

type KardiaManager struct {
	config         *config.ServiceConfig
	restClient     *tools.RestClient
	client         *ethclient.Client
	kaiclient      *kaiclient.Client
	currentHeight  uint64
	forceHeight    uint64
	lockerContract *bind.BoundContract
	polySdk        *sdk.PolySdk
	polySigner     *sdk.Account
	exitChan       chan int
	header4sync    [][]byte
	crosstx4sync   []*CrossTransfer
	db             *db.BoltDB
}

func NewKardiaManager(servconfig *config.ServiceConfig, startheight uint64, startforceheight uint64, ontsdk *sdk.PolySdk, ethclient *ethclient.Client, client *kaiclient.Client, boltDB *db.BoltDB) (*KardiaManager, error) {
	var wallet *sdk.Wallet
	var err error

	if !common.FileExisted(servconfig.PolyConfig.WalletFile) {
		wallet, err = ontsdk.CreateWallet(servconfig.PolyConfig.WalletFile)
		if err != nil {
			return nil, err
		}
	} else {
		wallet, err = ontsdk.OpenWallet(servconfig.PolyConfig.WalletFile)
		if err != nil {
			log.Errorf("NewKaiManager - wallet open error: %s", err.Error())
			return nil, err
		}
	}
	signer, err := wallet.GetDefaultAccount([]byte(servconfig.PolyConfig.WalletPwd))
	if err != nil || signer == nil {
		signer, err = wallet.NewDefaultSettingAccount([]byte(servconfig.PolyConfig.WalletPwd))
		if err != nil {
			log.Errorf("NewKaiManager - wallet password error")
			return nil, err
		}

		err = wallet.Save()
		if err != nil {
			return nil, err
		}
	}
	log.Infof("NewKaiManager - poly address: %s", signer.Address.ToBase58())

	mgr := &KardiaManager{
		config:        servconfig,
		exitChan:      make(chan int),
		currentHeight: startheight,
		forceHeight:   startforceheight,
		restClient:    tools.NewRestClient(),
		client:        ethclient,
		kaiclient:     client,
		polySdk:       ontsdk,
		polySigner:    signer,
		header4sync:   make([][]byte, 0),
		crosstx4sync:  make([]*CrossTransfer, 0),
		db:            boltDB,
	}
	err = mgr.init()
	if err != nil {
		return nil, err
	} else {
		return mgr, nil
	}
}

func (this *KardiaManager) MonitorChain() {
	fetchBlockTicker := time.NewTicker(config.KAI_MONITOR_INTERVAL)
	var blockHandleResult bool
	backtrace := uint64(1)
	for {
		select {
		case <-fetchBlockTicker.C:
			height, err := tools.GetNodeHeight(this.config.KAIConfig.RestURL, this.restClient)
			if err != nil {
				log.Infof("MonitorChain - cannot get node height, err: %s", err)
				continue
			}
			if height-this.currentHeight <= config.KAI_USEFUL_BLOCK_NUM {
				continue
			}
			log.Infof("MonitorChain - kai height is %d", height)
			blockHandleResult = true
			for this.currentHeight < height-config.KAI_USEFUL_BLOCK_NUM {
				blockHandleResult = this.handleNewBlock(this.currentHeight + 1)
				if !blockHandleResult {
					break
				}
				this.currentHeight++
				// try to commit header if more than 50 headers needed to be syned
				if len(this.header4sync) > 0 {
					if this.commitHeader() != 0 {
						log.Error("MonitorChain - commit header failed.", "height", this.currentHeight)
						blockHandleResult = false
						break
					}
					this.header4sync = make([][]byte, 0)
				}
			}
			if !blockHandleResult {
				continue
			}

			if len(this.header4sync) > 0 {
				// try to commit lastest header when we are at latest height
				commitHeaderResult := this.commitHeader()
				if commitHeaderResult > 0 {
					log.Error("MonitorChain - commit header failed.", "height", this.currentHeight)
					continue
				} else if commitHeaderResult == 0 {
					backtrace = 1
					this.header4sync = make([][]byte, 0)
					continue
				} else {
					latestHeight := this.findLastestHeight()
					if latestHeight == 0 {
						continue
					}
					this.currentHeight = latestHeight - backtrace
					backtrace++
					log.Errorf("MonitorChain - back to height: %d", this.currentHeight)
					this.header4sync = make([][]byte, 0)
				}
			}
		case <-this.exitChan:
			return
		}
	}
}
func (this *KardiaManager) init() error {
	// get latest height
	latestHeight := this.findLastestHeight()
	if latestHeight == 0 {
		return fmt.Errorf("init - the genesis block has not synced!")
	}
	log.Infof("init - latest synced height: %d", latestHeight)
	if this.forceHeight > 0 && this.forceHeight < latestHeight {
		this.currentHeight = this.forceHeight
	} else {
		this.currentHeight = latestHeight
	}
	// this.currentHeight = 760565
	return nil
}

func (this *KardiaManager) findLastestHeight() uint64 {
	// try to get key
	contractAddress := autils.HeaderSyncContractAddress
	key := append([]byte(scom.EPOCH_SWITCH), utils.GetUint64Bytes(this.config.KAIConfig.SideChainId)...)
	// try to get storage
	result, err := this.polySdk.GetStorage(contractAddress.ToHexString(), key)
	if err != nil {
		return 0
	}

	if len(result) == 0 {
		return 0
	} else {
		return binary.LittleEndian.Uint64(result)
	}
}

func (this *KardiaManager) handleNewBlock(height uint64) bool {
	ret := this.handleBlockHeader(height)
	if !ret {
		log.Errorf("handleNewBlock - handleBlockHeader on height :%d failed", height)
		return false
	}
	ret = this.fetchLockDepositEvents(height-1, this.client)
	if !ret {
		log.Errorf("handleNewBlock - fetchLockDepositEvents on height :%d failed", height)
	}
	return true
}

func (this *KardiaManager) handleBlockHeader(height uint64) bool {
	ctx := context.Background()
	number := big.NewInt(int64(height))
	header, err := this.kaiclient.HeaderByNumber(ctx, number)
	if err != nil {
		log.Error("handleBlockHeader - GetNodeHeader on height :%d failed", height, "err", err)
		return false
	}

	if header.ValidatorsHash.Equal(header.NextValidatorsHash) {
		return true
	}

	val, _ := this.polySdk.GetStorage(utils.CrossChainManagerContractAddress.ToHexString(),
		append(append([]byte(scom.EPOCH_SWITCH), utils.GetUint64Bytes(this.config.KAIConfig.SideChainId)...),
			utils.GetUint64Bytes(uint64(header.Height))...))
	// check if this header is not committed on Poly
	if len(val) > 0 {
		return true
	}

	validators, err := this.kaiclient.GetValidators(ctx, number)
	if err != nil {
		log.Error("handleBlockHeader - GetValidators on height :%d failed", height, "err", err)
		return false
	}

	commit, err := this.kaiclient.GetCommit(ctx, number.Sub(number, big.NewInt(1)))
	if err != nil {
		log.Error("handleBlockHeader - GetCommit on height :%d failed", height, "err", err)
		return false
	}

	fullHeader := &kaiclient.KaiHeader{
		Header:       header,
		ValidatorSet: validators,
		Commit:       commit,
	}
	headerBytes, err := json.Marshal(fullHeader)
	if err != nil {
		log.Errorf("marshal header on height :%d failed err %s", height, err)
		return false
	}
	this.header4sync = append(this.header4sync, headerBytes)
	return true
}

func (this *KardiaManager) fetchLockDepositEvents(height uint64, client *ethclient.Client) bool {
	lockAddress := ethcommon.HexToAddress(this.config.KAIConfig.ECCMContractAddress)
	lockContract, err := eccm_abi.NewEthCrossChainManager(lockAddress, client)
	if err != nil {
		return false
	}
	opt := &bind.FilterOpts{
		Start:   height,
		End:     &height,
		Context: context.Background(),
	}
	events, err := lockContract.FilterCrossChainEvent(opt, nil)
	if err != nil {
		log.Errorf("fetchLockDepositEvents - FilterCrossChainEvent error :%s", err.Error())
		return false
	}
	if events == nil {
		log.Infof("fetchLockDepositEvents - no events found on FilterCrossChainEvent")
		return false
	}
	for events.Next() {
		evt := events.Event
		index := big.NewInt(0)
		index.SetBytes(evt.TxId)
		crossTx := &CrossTransfer{
			txIndex: tools.EncodeBigInt(index),
			txId:    evt.Raw.TxHash.Bytes(),
			toChain: uint32(evt.ToChainId),
			value:   []byte(evt.Rawdata),
			height:  height,
		}
		sink := common.NewZeroCopySink(nil)
		crossTx.Serialization(sink)
		err = this.db.PutRetry(sink.Bytes())
		if err != nil {
			log.Errorf("fetchLockDepositEvents - this.db.PutRetry error: %s", err)
		}
		log.Infof("fetchLockDepositEvent -  height: %d", height)
	}
	return true
}

func (this *KardiaManager) commitHeader() int {
	tx, err := this.polySdk.Native.Hs.SyncBlockHeader(
		this.config.KAIConfig.SideChainId,
		this.polySigner.Address,
		this.header4sync,
		this.polySigner,
	)
	if err != nil {
		log.Warnf("commitHeader - send transaction to poly chain err: %s!", err.Error())
		errDesc := err.Error()
		if strings.Contains(errDesc, "get the parent block failed") || strings.Contains(errDesc, "missing required field") {
			return -1
		} else {
			return 1
		}
	}
	tick := time.NewTicker(100 * time.Millisecond)
	var h uint32
	for range tick.C {
		h, _ = this.polySdk.GetBlockHeightByTxHash(tx.ToHexString())
		curr, _ := this.polySdk.GetCurrentBlockHeight()
		if h > 0 && curr > h {
			break
		}
	}
	log.Infof("commitHeader - send transaction %s to poly chain and confirmed on height %d", tx.ToHexString(), h)
	return 0
}
func (this *KardiaManager) MonitorDeposit() {
	monitorTicker := time.NewTicker(config.KAI_MONITOR_INTERVAL)
	for {
		select {
		case <-monitorTicker.C:
			if err := this.handleLockDepositEvents(); err != nil {
				log.Errorf("MonitorChain - handleLockDepositEvents, err: %s", err)
			}
		case <-this.exitChan:
			return
		}
	}
}
func (this *KardiaManager) handleLockDepositEvents() error {
	retryList, err := this.db.GetAllRetry()
	if err != nil {
		return fmt.Errorf("handleLockDepositEvents - this.db.GetAllRetry error: %s", err)
	}
	fmt.Println("----------------->", len(retryList))
	for _, v := range retryList {
		time.Sleep(time.Second * 1)
		crosstx := new(CrossTransfer)
		err := crosstx.Deserialization(common.NewZeroCopySource(v))
		if err != nil {
			log.Errorf("handleLockDepositEvents - retry.Deserialization error: %s", err)
			continue
		}
		//1. decode events
		key := crosstx.txIndex
		keyBytes, err := eth.MappingKeyAt(key, "01")
		if err != nil {
			log.Errorf("handleLockDepositEvents - MappingKeyAt error:%s\n", err.Error())
			continue
		}

		heightHex := hexutil.EncodeBig(big.NewInt(int64(crosstx.height + 1)))
		proofKey := hexutil.Encode(keyBytes)
		//2. get proof
		proof, err := tools.GetProof(this.config.KAIConfig.RestURL, this.config.KAIConfig.ECCDContractAddress, proofKey, heightHex, this.restClient)
		if err != nil {
			log.Errorf("handleLockDepositEvents - error :%s\n", err.Error())
			continue
		}
		//3. commit proof to poly
		txHash, err := this.commitProof(uint32(crosstx.height+1), proof, crosstx.value, crosstx.txId)
		if err != nil {
			if strings.Contains(err.Error(), "chooseUtxos, current utxo is not enough") {
				log.Infof("handleLockDepositEvents - invokeNativeContract error: %s", err)
				continue
			} else {
				if err := this.db.DeleteRetry(v); err != nil {
					log.Errorf("handleLockDepositEvents - this.db.DeleteRetry error: %s", err)
				}
				log.Errorf("handleLockDepositEvents - invokeNativeContract error: %s", err)
				continue
			}
		}
		//4. put to check db for checking
		err = this.db.PutCheck(txHash, v)
		if err != nil {
			log.Errorf("handleLockDepositEvents - this.db.PutCheck error: %s", err)
		}
		err = this.db.DeleteRetry(v)
		if err != nil {
			log.Errorf("handleLockDepositEvents - this.db.PutCheck error: %s", err)
		}
		log.Infof("handleLockDepositEvents - syncProofToAlia txHash is %s", txHash)
	}
	return nil
}
func (this *KardiaManager) commitProof(height uint32, proof []byte, value []byte, txhash []byte) (string, error) {
	ctx := context.Background()
	header, err := this.kaiclient.FullHeaderByNumber(ctx, big.NewInt(int64(height)))
	if err != nil {
		return "", err
	}

	headerBytes, err := json.Marshal(header)
	if err != nil {
		return "", err
	}

	log.Infof("commit proof, height: %d, proof: %s, value: %s, txhash: %s", height, string(proof), hex.EncodeToString(value), hex.EncodeToString(txhash))
	tx, err := this.polySdk.Native.Ccm.ImportOuterTransfer(
		this.config.KAIConfig.SideChainId,
		value,
		height,
		proof,
		ethcommon.Hex2Bytes(this.polySigner.Address.ToHexString()),
		headerBytes,
		this.polySigner)
	if err != nil {
		return "", err
	} else {
		log.Infof("commitProof - send transaction to poly chain: %s, height: %d", tx.ToHexString(), height)
		return tx.ToHexString(), nil
	}
}

func (this *KardiaManager) CheckDeposit() {
	checkTicker := time.NewTicker(config.KAI_MONITOR_INTERVAL)
	for {
		select {
		case <-checkTicker.C:
			// try to check deposit
			_ = this.checkLockDepositEvents()
		case <-this.exitChan:
			return
		}
	}
}
func (this *KardiaManager) checkLockDepositEvents() error {
	checkMap, err := this.db.GetAllCheck()
	if err != nil {
		return fmt.Errorf("checkLockDepositEvents - this.db.GetAllCheck error: %s", err)
	}
	for k, v := range checkMap {
		time.Sleep(time.Second * 1)
		event, err := this.polySdk.GetSmartContractEvent(k)
		if err != nil {
			return fmt.Errorf("checkLockDepositEvents - this.aliaSdk.GetSmartContractEvent error: %s", err)
		}
		if event == nil {
			log.Infof("checkLockDepositEvents - can not find event of hash %s", k)
			continue
		}
		if event.State != 1 {
			log.Infof("checkLockDepositEvents - state of tx %s is not success", k)
			err := this.db.PutRetry(v)
			if err != nil {
				log.Errorf("checkLockDepositEvents - this.db.PutRetry error:%s", err)
			}
		} else {
			err := this.db.DeleteCheck(k)
			if err != nil {
				log.Errorf("checkLockDepositEvents - this.db.DeleteRetry error:%s", err)
			}
		}
	}
	return nil
}
