package manage

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/ethereum/go-ethereum/ethclient"
)

var (
	Client             *ethclient.Client
	lastConnectionTime int64
)

// ConnNode connects to a node, simple pimple. client can be used asyncronously.
func ConnNode(node string) (client *ethclient.Client, err error) {
	if Client != nil {
		return Client, nil
	}
	client, err = ethclient.Dial(node)
	if err != nil {
		errors.Wrapf(err, fmt.Sprintf("::Fatal:: Could not connect to node:", node))
		return client, err
	}
	Client = client
	return client, nil
}

// InitAuth grabs the basics to get goin and stores them in the AuthInfo type
func Boot(node string) (client *ethclient.Client, err error) {
	client, err = ConnNode(node)
	if err != nil {
		errors.New("--Could not connect to node-- ::Fatal::")
		return nil, err
	}

	return client, nil
}
