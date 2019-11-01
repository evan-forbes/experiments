package uniswap

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"time"

	"github.com/pkg/errors"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/evan-forbes/svrd/contracts/erc20"
)

// still adapting this old code to this project, so for now, everything is commented out

const (
	HexFactoryAddr = "0xc0a47dFe034B400B47bDaD5FecDa2621de6c4d95"
)

var (
	oneQuint  = big.NewInt(1000000000000000000)
	bigFee    = big.NewInt(30000000000000000)
	noOptions = &bind.CallOpts{}
)

type Cache struct {
	Prices map[string]*ExchangeLog
}

//////////// Connecting to the Exchange //////////////
// InitExch creates the contract object for a given Uniswap Exchange Address
func InitExch(client *ethclient.Client, exchAddr, name string) (contract *Uniswap, err error) {
	address := common.HexToAddress(exchAddr)
	contract, err = NewUniswap(address, client)
	return contract, nil
}

/////////////////////////////////////
//////////// Exchange //////////////

type ExchangeLog struct {
	Ticker      string `json:"ticker"`
	Addr        string `json:"address"`
	TokenPool   string `json:"tokenPool"`
	EthPool     string `json:"ethPool"`
	Invariant   string `json:"invariant"`
	TokenPerEth string `json:"tokenPerEth"`
	EthPerToken string `json:"tokenPerEth"`
	TokenSpread string `json:"tokenSpread"`
	EthSpread   string `json:"ethPerToken"`
	Timestamp   int64  `json:"time"`
}

func (exch *ExchangeLog) Print() {
	fmt.Printf("\n%s \n%s %s \n%s %s \n%s %s \n",
		"Uniswap Prices:", "TokenPerEth", exch.TokenPerEth,
		"EthPerToken:", exch.EthPerToken,
		"Spread:", exch.TokenSpread)
}

func (exch *Exchange) MakeLog() ExchangeLog {
	return ExchangeLog{
		Ticker:      exch.Ticker,
		Addr:        hex.EncodeToString(exch.Addr.Bytes()),
		TokenPool:   exch.TokenPool.String(),
		EthPool:     exch.EthPool.String(),
		Invariant:   exch.Invariant.String(),
		TokenPerEth: exch.TokenPerEth.String(),
		EthPerToken: exch.EthPerToken.String(),
		TokenSpread: exch.TokenSpread.String(),
		EthSpread:   exch.EthSpread.String(),
		Timestamp:   exch.Timestamp,
	}
}

// Exchange contains all the info and methods
// needed to use a uniswap exchange
type Exchange struct {
	Ticker                   string
	Addr                     common.Address
	Erc20                    *erc20.Erc20
	TokenPool, EthPool       *big.Int
	Invariant                *big.Int
	TokenPerEth, EthPerToken *big.Int
	TokenSpread, EthSpread   *big.Int
	Timestamp                int64
}

// NewExch makes a new exchange at the give address and updates the pools
func NewExch(client *ethclient.Client, ticker string, addr, erc20addr common.Address) (*Exchange, error) {
	erc, err := erc20.NewErc20(erc20addr, client)
	if err != nil {
		return nil, errors.Wrapf(err, "could not establish new uniswap exchange %s", ticker)
	}
	exch := Exchange{
		Ticker: ticker,
		Addr:   addr,
		Erc20:  erc,
	}
	return &exch, nil
}

// UpdatePools talks to the Uniswap exchange contract and the erc20
// it represents to get the pool sizes, then stores them in
// the exchange type.
func (exch *Exchange) UpdatePools(client *ethclient.Client, ctx context.Context) (err error) {
	exch.EthPool, err = client.BalanceAt(ctx, exch.Addr, nil)
	if err != nil {
		fmt.Println("Could not talk to exch contract", err)
		return err
	}

	exch.TokenPool, err = exch.Erc20.BalanceOf(noOptions, exch.Addr)
	if err != nil {
		fmt.Println("Could not talk to erc20 contract", err)
		return err
	}
	exch.UpdateInv()
	exch.Timestamp = time.Now().Unix()
	return nil
}

// UpdateInv simply multiplies the pool sizes to calc the invariant
// only used in UdatePools.
func (exch *Exchange) UpdateInv() {
	exch.Invariant = new(big.Int).Mul(exch.EthPool, exch.TokenPool)
}

// UpdateperUnitPrices does some tedious calculations to show the output of a single eth/token
// along with finding out the "spread" (the value lost reversing a swap) in terms of eth and
// the token. This function does not alter the exchange pools.
func (exch *Exchange) UpdatePerUnitPrices() {
	exch.TokenPerEth = CalcOutput(oneQuint, exch.EthPool, exch.TokenPool)
	exch.EthPerToken = CalcOutput(oneQuint, exch.TokenPool, exch.EthPool)
	exch.EthSpread = new(big.Int).Sub(oneQuint, CalcOutput(exch.TokenPerEth, exch.TokenPool, exch.EthPool))
	exch.TokenSpread = new(big.Int).Div(new(big.Int).Mul(exch.EthSpread, exch.TokenPerEth), oneQuint)
}

// Swap changes the pool sizes by the given amount inputA
func Swap(inputA, poolA, poolB *big.Int) (newPoolA, newPoolB, output *big.Int) {
	inv := new(big.Int).Mul(poolA, poolB)
	fee, remain := calcFee(inputA)
	tempPoolA := poolA.Add(poolA, remain)
	newPoolB = new(big.Int).Div(inv, tempPoolA)
	output = new(big.Int).Sub(poolB, newPoolB)
	newPoolA = new(big.Int).Add(tempPoolA, fee)
	return newPoolA, newPoolB, output
}

// SwapToken takes tokens and changes pools of an exchange to reflect an
// actual swap
func (exch *Exchange) SwapToken(input *big.Int) (output *big.Int) {
	output, exch.TokenPool, exch.EthPool = Swap(input, exch.TokenPool, exch.EthPool)
	return output
}

// SwapEth takes eth and changes pools of an exchange to reflect an
// actual swap
func (exch *Exchange) SwapEth(input *big.Int) (output *big.Int) {
	output, exch.EthPool, exch.TokenPool = Swap(input, exch.EthPool, exch.TokenPool)
	return output
}

// CalcOutput finds the output given an input amount into a uniswap
// exchange. NOTE: this does not actually *change an exchange type*
// see the Swap/SwapToken/SwapEth functions for more accurate changes
// to an exchange.
func CalcOutput(inputA, poolA, poolB *big.Int) *big.Int {
	inv := new(big.Int).Mul(poolA, poolB)
	_, remain := calcFee(inputA)
	newPoolA := poolA.Add(poolA, remain)
	newPoolB := new(big.Int).Div(inv, newPoolA)
	out := new(big.Int).Sub(poolB, newPoolB)
	return out
}

func calcFee(input *big.Int) (fee *big.Int, remain *big.Int) {
	fee = new(big.Int).Div(input, big.NewInt(333))
	remain = new(big.Int).Sub(input, fee)
	return
}

func bigMult(x *big.Int, y float64) *big.Int {
	flout := new(big.Float)
	a := new(big.Float).SetInt(x)
	b := big.NewFloat(y)
	flout.Mul(a, b)
	out, _ := flout.Int(nil)
	return out
}

func calcPrice(amtA, amtB *big.Int) *big.Int {
	return new(big.Int).Div(amtA, amtB)
}

// // still need to test this function out
// func (exch *Exchange) EthToToken(ethAmt *big.Int) *big.Int {
// 	fee, remain := calcFee(ethAmt)
// 	exch.EthPool.Add(exch.EthPool, remain)
// 	newTokenPool := new(big.Int).Div(exch.Invariant, exch.EthPool)
// 	out := exch.TokenPool.Sub(exch.TokenPool, newTokenPool)
// 	exch.EthPool.Add(exch.EthPool, fee)
// 	exch.TokenPool = newTokenPool
// 	exch.UpdateInv()
// 	return out
// }

// func (exch *Exchange) EthToToken(ethAmt *big.Int) *big.Int {
// 	fmt.Println("old eth amount", ethAmt)
// 	fee := bigMult(ethAmt, .003)
// 	fmt.Println("fee: ", fee)
// 	ethAmt.Sub(ethAmt, fee)
// 	fmt.Println("new eth amount", ethAmt)
// 	exch.EthPool.Add(exch.EthPool, ethAmt)
// 	newTokenPool := new(big.Int).Div(exch.Invariant, exch.EthPool)
// 	fmt.Println("new token pool:", newTokenPool)
// 	return new(big.Int).Sub(exch.TokenPool, newTokenPool)
// }
