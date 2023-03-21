package main

import (
	"fmt"
	"six-data-chain-go-sdk/api"

	// "github.com/thesixnetwork/six-data-chain-go-sdk/api"

	"github.com/google/uuid"
	nftmngrtypes "github.com/thesixnetwork/sixnft/x/nftmngr/types"
)

func main() {
	nodeURL := "YOUR NODE URL"
	armor := `
-----BEGIN TENDERMINT PRIVATE KEY-----
YOUR KEY DETAIL
-----END TENDERMINT PRIVATE KEY-----
	`
	passphrase := "YOUR PASSPHARSE"
	importAccountName := "YOUR ACCOUNT NAME"
	chainID := "YOUR CHAIN ID"

	// Create a new API client
	gasPrice := "1.25usix" // default "1.25usix"
	clientOptions := api.ClientOptions{
		BroadcastMode: "async", // default "block"
		GasPrices:     &gasPrice,
	}
	client, err := api.NewClient(
		nodeURL,
		armor,
		passphrase,
		importAccountName,
		chainID,
		&clientOptions,
	)
	if err != nil {
		fmt.Println(err)
		return
	}

	refID := uuid.New()
	msg := &nftmngrtypes.MsgPerformActionByAdmin{
		Creator:       client.ConnectedAddress,
		NftSchemaCode: "six-protocol.develop_v220",
		TokenId:       "1",
		Action:        "test_read_nft",
		RefId:         refID.String(),
		Parameters:    []*nftmngrtypes.ActionParameter{},
	}

	txResponse, err := client.GenerateOrBroadcastTx(msg)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("txResponse code", txResponse.Code)
	fmt.Println("txResponse hash", txResponse.TxHash)

	fmt.Println()

	queryClient := client.QueryClient()
	fmt.Println("queryClient: ", queryClient)
	fmt.Println()

}
