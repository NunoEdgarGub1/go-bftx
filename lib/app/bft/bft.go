// File: ./blockfreight/lib/bft.go
// Summary: Application code for Blockfreight™ | The blockchain of global freight.
// License: MIT License
// Company: Blockfreight, Inc.
// Author: Julian Nunez, Neil Tran, Julian Smith, Gian Felipe & contributors
// Site: https://blockfreight.com
// Support: <support@blockfreight.com>

// Copyright © 2017 Blockfreight, Inc. All Rights Reserved.

// Permission is hereby granted, free of charge, to any person obtaining
// a copy of this software and associated documentation files (the "Software"),
// to deal in the Software without restriction, including without limitation
// the rights to use, copy, modify, merge, publish, distribute, sublicense,
// and/or sell copies of the Software, and to permit persons to whom the
// Software is furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be
// included in all copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS
// OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY,
// WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
// CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

// =================================================================================================================================================
// =================================================================================================================================================
//
// BBBBBBBBBBBb     lll                                kkk             ffff                         iii                  hhh            ttt
// BBBB``````BBBB   lll                                kkk            fff                           ```                  hhh            ttt
// BBBB      BBBB   lll      oooooo        ccccccc     kkk    kkkk  fffffff  rrr  rrr    eeeee      iii     gggggg ggg   hhh  hhhhh   tttttttt
// BBBBBBBBBBBB     lll    ooo    oooo    ccc    ccc   kkk   kkk    fffffff  rrrrrrrr eee    eeee   iii   gggg   ggggg   hhhh   hhhh  tttttttt
// BBBBBBBBBBBBBB   lll   ooo      ooo   ccc           kkkkkkk        fff    rrrr    eeeeeeeeeeeee  iii  gggg      ggg   hhh     hhh    ttt
// BBBB       BBB   lll   ooo      ooo   ccc           kkkk kkkk      fff    rrr     eeeeeeeeeeeee  iii   ggg      ggg   hhh     hhh    ttt
// BBBB      BBBB   lll   oooo    oooo   cccc    ccc   kkk   kkkk     fff    rrr      eee      eee  iii    ggg    gggg   hhh     hhh    tttt    ....
// BBBBBBBBBBBBB    lll     oooooooo       ccccccc     kkk     kkkk   fff    rrr       eeeeeeeee    iii     gggggg ggg   hhh     hhh     ttttt  ....
//                                                                                                        ggg      ggg
//   Blockfreight™ | The blockchain of global freight.                                                      ggggggggg
//
// =================================================================================================================================================
// =================================================================================================================================================

// Package bft implements the main functions to work with the Blockfreight™ Network.
package bft

import (
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/blockfreight/go-bftx/lib/app/bf_tx"
	// =======================
	// Golang Standard library
	// =======================
	// Implements simple functions to manipulate UTF-8 encoded strings.

	// ===============
	// Tendermint Core
	// ===============

	"bytes"
	"fmt"

	"github.com/tendermint/abci/example/code"
	"github.com/tendermint/abci/types"
	crypto "github.com/tendermint/go-crypto"
	"github.com/tendermint/iavl"
	dbm "github.com/tendermint/tmlibs/db"

	wire "github.com/tendermint/go-wire"
)

const (
	ValidatorSetChangePrefix string = "val:"
)

// BftApplication struct
type BftApplication struct {
	types.BaseApplication

	state *iavl.VersionedTree

	// validator set
	ValUpdates []*types.Validator
}

// NewBftApplication creates a new application
func NewBftApplication() *BftApplication {
	stateTree := iavl.NewVersionedTree(0, dbm.NewDB("tendermint", dbm.LevelDBBackendStr, "./db"))

	return &BftApplication{
		state: stateTree,
	}
}

// Info returns information
func (app *BftApplication) Info(req types.RequestInfo) (resInfo types.ResponseInfo) {
	return types.ResponseInfo{Data: fmt.Sprintf(`{"size":%v}`, app.state.Size()), LastBlockAppHash: app.state.Hash()}
}

// DeliverTx delivers transactions.Transactions are either "key=value" or just arbitrary bytes
func (app *BftApplication) DeliverTx(tx []byte) types.ResponseDeliverTx {

	if isValidatorTx(tx) {
		// update validators in the merkle tree
		// and in app.ValUpdates
		return app.execValidatorTx(tx)
	}

	var key, value []byte
	parts := bytes.Split(tx, []byte("="))
	if len(parts) == 2 {
		key, value = parts[0], parts[1]
	} else {
		key, value = tx, tx
	}
	app.state.Set(key, value)

	var bftx bf_tx.BF_TX
	err := json.Unmarshal(tx, &bftx)
	if err != nil {
		// if this wasn't a dummy app, we'd do something smarter
		panic(err)
	}

	tags := []*types.KVPair{
		{Key: "bftx.id", ValueType: types.KVPair_STRING, ValueString: bftx.Id},
	}
	return types.ResponseDeliverTx{Code: code.CodeTypeOK, Tags: tags}
}

// CheckTx checks a transaction
func (app *BftApplication) CheckTx(tx []byte) types.ResponseCheckTx {
	return types.ResponseCheckTx{Code: code.CodeTypeOK}
}

// Commit commits transactions
func (app *BftApplication) Commit() types.ResponseCommit {
	// Save a new version
	var hash []byte
	var err error

	if app.state.Size() > 0 {
		// just add one more to height (kind of arbitrarily stupid)
		height := app.state.LatestVersion() + 1
		hash, err = app.state.SaveVersion(height)
		if err != nil {
			// if this wasn't a dummy app, we'd do something smarter
			panic(err)
		}
	}

	return types.ResponseCommit{Code: code.CodeTypeOK, Data: hash}
}

//Query retrieves a transaction from the network
func (app *BftApplication) Query(reqQuery types.RequestQuery) (resQuery types.ResponseQuery) {
	if reqQuery.Prove {
		value, proof, err := app.state.GetWithProof(reqQuery.Data)
		// if this wasn't a dummy app, we'd do something smarter
		if err != nil {
			panic(err)
		}
		resQuery.Index = -1 // TODO make Proof return index
		resQuery.Key = reqQuery.Data
		resQuery.Value = value
		resQuery.Proof = wire.BinaryBytes(proof)
		if value != nil {
			resQuery.Log = "exists"
		} else {
			resQuery.Log = "does not exist"
		}
		return
	} else {
		index, value := app.state.Get(reqQuery.Data)
		resQuery.Index = int64(index)
		resQuery.Value = value
		if value != nil {
			resQuery.Log = "exists"
		} else {
			resQuery.Log = "does not exist"
		}
		return
	}
}

// Save the validators in the merkle tree
func (app *BftApplication) InitChain(req types.RequestInitChain) types.ResponseInitChain {
	for _, v := range req.Validators {
		r := app.updateValidator(v)
		if r.IsErr() {
			panic(r)
		}
	}
	return types.ResponseInitChain{}
}

// Track the block hash and header information
func (app *BftApplication) BeginBlock(req types.RequestBeginBlock) types.ResponseBeginBlock {
	// reset valset changes
	app.ValUpdates = make([]*types.Validator, 0)
	return types.ResponseBeginBlock{}
}

// Update the validator set
func (app *BftApplication) EndBlock(req types.RequestEndBlock) types.ResponseEndBlock {
	return types.ResponseEndBlock{ValidatorUpdates: app.ValUpdates}
}

func isValidatorTx(tx []byte) bool {
	return strings.HasPrefix(string(tx), ValidatorSetChangePrefix)
}

func (app *BftApplication) execValidatorTx(tx []byte) types.ResponseDeliverTx {
	tx = tx[len(ValidatorSetChangePrefix):]

	//get the pubkey and power
	pubKeyAndPower := strings.Split(string(tx), "/")
	if len(pubKeyAndPower) != 2 {
		return types.ResponseDeliverTx{
			Code: code.CodeTypeEncodingError,
			Log:  fmt.Sprintf("Expected 'pubkey/power'. Got %v", pubKeyAndPower)}
	}
	pubkeyS, powerS := pubKeyAndPower[0], pubKeyAndPower[1]

	// decode the pubkey, ensuring its go-crypto encoded
	pubkey, err := hex.DecodeString(pubkeyS)
	if err != nil {
		return types.ResponseDeliverTx{
			Code: code.CodeTypeEncodingError,
			Log:  fmt.Sprintf("Pubkey (%s) is invalid hex", pubkeyS)}
	}
	_, err = crypto.PubKeyFromBytes(pubkey)
	if err != nil {
		return types.ResponseDeliverTx{
			Code: code.CodeTypeEncodingError,
			Log:  fmt.Sprintf("Pubkey (%X) is invalid go-crypto encoded", pubkey)}
	}

	// decode the power
	power, err := strconv.ParseInt(powerS, 10, 64)
	if err != nil {
		return types.ResponseDeliverTx{
			Code: code.CodeTypeEncodingError,
			Log:  fmt.Sprintf("Power (%s) is not an int", powerS)}
	}

	// update
	return app.updateValidator(&types.Validator{pubkey, power})
}

// add, update, or remove a validator
func (app *BftApplication) updateValidator(v *types.Validator) types.ResponseDeliverTx {
	key := []byte("val:" + string(v.PubKey))
	if v.Power == 0 {
		// remove validator
		if !app.state.Has(key) {
			return types.ResponseDeliverTx{
				Code: code.CodeTypeUnauthorized,
				Log:  fmt.Sprintf("Cannot remove non-existent validator %X", key)}
		}
		app.state.Remove(key)
	} else {
		// add or update validator
		value := bytes.NewBuffer(make([]byte, 0))
		if err := types.WriteMessage(v, value); err != nil {
			return types.ResponseDeliverTx{
				Code: code.CodeTypeEncodingError,
				Log:  fmt.Sprintf("Error encoding validator: %v", err)}
		}
		app.state.Set(key, value.Bytes())
	}

	// we only update the changes array if we successfully updated the tree
	app.ValUpdates = append(app.ValUpdates, v)

	return types.ResponseDeliverTx{Code: code.CodeTypeOK}
}

// =================================================
// Blockfreight™ | The blockchain of global freight.
// =================================================

// BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB
// BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB
// BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB
// BBBBBBB                    BBBBBBBBBBBBBBBBBBB
// BBBBBBB                       BBBBBBBBBBBBBBBB
// BBBBBBB                        BBBBBBBBBBBBBBB
// BBBBBBB       BBBBBBBBB        BBBBBBBBBBBBBBB
// BBBBBBB       BBBBBBBBB        BBBBBBBBBBBBBBB
// BBBBBBB       BBBBBBB         BBBBBBBBBBBBBBBB
// BBBBBBB                     BBBBBBBBBBBBBBBBBB
// BBBBBBB                        BBBBBBBBBBBBBBB
// BBBBBBB       BBBBBBBBBB        BBBBBBBBBBBBBB
// BBBBBBB       BBBBBBBBBBB       BBBBBBBBBBBBBB
// BBBBBBB       BBBBBBBBBB        BBBBBBBBBBBBBB
// BBBBBBB       BBBBBBBBB        BBB       BBBBB
// BBBBBBB                       BBBB       BBBBB
// BBBBBBB                    BBBBBBB       BBBBB
// BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB
// BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB
// BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB

// ==================================================
// Blockfreight™ | The blockchain for global freight.
// ==================================================
