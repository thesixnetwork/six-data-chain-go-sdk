package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/thesixnetwork/six-protocol-go-sdk/api"
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
	chainID := "YOUR CHAIN ID"

	// Create a new API client
	gasPrice := "1.25usix" // default "1.25usix"
	clientOptions := &api.ClientOptions{
		BroadcastMode: "async", // default "block"
		GasPrices:     &gasPrice,
	}
	client, err := api.NewClient(
		nodeURL,
		armor,
		passphrase,
		chainID,
		clientOptions,
	)
	if err != nil {
		fmt.Println(err)
		return
	}

	// Create NFT Schema
	// Read JSON data from file
	jsonData, err := ioutil.ReadFile("json/schema.json")
	if err != nil {
		fmt.Println(err)
		return
	}

	// Decode JSON data into a map[string]interface{} variable
	var data map[string]interface{}
	err = json.Unmarshal(jsonData, &data)
	if err != nil {
		fmt.Println(err)
		return
	}

	// Encode the JSON data as a base64 string
	base64Data := base64.StdEncoding.EncodeToString(jsonData)

	msg := &nftmngrtypes.MsgCreateNFTSchema{
		Creator:         client.ConnectedAddress,
		NftSchemaBase64: base64Data,
	}

	txResponse, err := client.GenerateOrBroadcastTx(msg)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("txResponse code", txResponse.Code)
	fmt.Println("txResponse hash", txResponse.TxHash)
}
