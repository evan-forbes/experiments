package main

import (
	"context"
	"fmt"
	"testing"
)

func TestMain(t *testing.T) {
	Init()
	i := Input{
		Name:         "DAI",
		Address:      "0x09cabEC1eAd1c0Ba254B09efb3EE13841712bE14",
		Erc20Address: "0x89d24a6b4ccb1b6faa2625fe562bdd9a23260359"}
	l, err := HandleRequest(context.Background(), i)
	if err != nil {
		t.Error(err)
	}
	t.Log(l)
	ll, has := local.read("0x09cabEC1eAd1c0Ba254B09efb3EE13841712bE14")
	if !has {
		t.Error("did not write locally after fetching data")
	}
	if ll.EthPerToken != l.EthPerToken {
		t.Errorf("two different token amounts %s vs %s", ll.EthPerToken, l.EthPerToken)
	}
	if l.EthPerToken == "" {
		t.Error("no eth per token calculated")
	}
	fmt.Println(HandleRequest(context.Background(), i))
}
