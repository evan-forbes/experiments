package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/evan-forbes/practice/lambda/uniswapPriceFetch/uniswap"
	"github.com/pkg/errors"
)

const (
	nodeAddr = "wss://mainnet.infura.io/ws"
)

type cache struct {
	data map[string]uniswap.ExchangeLog
	mut  sync.RWMutex
}

func newCache() *cache {
	return &cache{
		data: make(map[string]uniswap.ExchangeLog),
		mut:  sync.RWMutex{},
	}
}

func (c *cache) read(addr string) (uniswap.ExchangeLog, bool) {
	c.mut.Lock()
	out, has := c.data[addr]
	c.mut.Unlock()
	return out, has
}

func (c *cache) write(key string, l uniswap.ExchangeLog) {
	c.mut.Lock()
	c.data[key] = l
	c.mut.Unlock()
}

var (
	Client *ethclient.Client
	local  *cache
)

func Init() {
	if Client == nil {
		client, err := connNode(nodeAddr)
		if err != nil {
			log.Fatal("Could not connect to Ethereum Node")
		}
		Client = client
	}
	if local == nil {
		local = newCache()
	}
}

func init() {
	if Client == nil {
		client, err := connNode(nodeAddr)
		if err != nil {
			log.Fatal("Could not connect to Ethereum Node")
		}
		Client = client
	}
	if local == nil {
		local = newCache()
	}
}

// ConnNode connects to a node, simple pimple. client can be used asyncronously.
func connNode(node string) (client *ethclient.Client, err error) {
	client, err = ethclient.Dial(node)
	if err != nil {
		errors.Wrapf(err, fmt.Sprintf("::Fatal:: Could not connect to node:%s", node))
		return client, err
	}
	Client = client
	return client, nil
}

type Input struct {
	Name         string `json:"name"`
	Address      string `json:"address"`
	Erc20Address string `json:"erc20Address"`
}

func HandleRequest(ctx context.Context, in Input) (out uniswap.ExchangeLog, err error) {
	// check for a valid address
	if !common.IsHexAddress(in.Address) || !common.IsHexAddress(in.Erc20Address) {
		return out, errors.New("::Fatal:: invalid address")
	}
	out, err = fetchData(Client, ctx, in)
	if err != nil {
		return out, errors.Wrapf(err, "could not fetch data for contracts")
	}
	local.write(in.Address, out)
	return out, nil
}

func fetchData(client *ethclient.Client, ctx context.Context, in Input) (out uniswap.ExchangeLog, err error) {
	// check to see if any relevant data is cached
	l, has := local.read(in.Address)
	if has {
		age := time.Now().Unix() - l.Timestamp // age in seconds
		if age < 24 {
			log.Println("served cached result")
			return l, nil
		}
	}
	exchange, err := uniswap.NewExch(
		client,
		in.Name,
		common.HexToAddress(in.Address),
		common.HexToAddress(in.Erc20Address),
	)
	if err != nil {
		return out, err
	}
	err = exchange.UpdatePools(client, ctx)
	if err != nil {
		return out, err
	}
	exchange.UpdatePerUnitPrices()
	out = exchange.MakeLog()
	fmt.Println(out)
	return out, nil
}

func main() {
	lambda.Start(HandleRequest)
}
