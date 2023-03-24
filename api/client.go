package api

import (
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/simapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/google/uuid"

	"github.com/cosmos/cosmos-sdk/x/auth/types"
	nftmngrtypes "github.com/thesixnetwork/sixnft/x/nftmngr/types"

	cliflags "github.com/cosmos/cosmos-sdk/client/flags"
	flag "github.com/spf13/pflag"
	rpchttp "github.com/tendermint/tendermint/rpc/client/http"
)

type BroadcastMode string

const (
	BroadcastAsync BroadcastMode = cliflags.BroadcastAsync
	BroadcastSync  BroadcastMode = cliflags.BroadcastSync
	BroadcastBlock BroadcastMode = cliflags.BroadcastBlock
)

type ClientOptions struct {
	BroadcastMode BroadcastMode
	GasPrices     *string
}

var DefaultGasPrice = "1.25usix"

const prefix = "6x"

// Client is a struct representing the API client.
type Client struct {
	nodeURL           string
	armor             string
	passphrase        string
	importAccountName string
	cosmosClient      *rpchttp.HTTP
	kr                keyring.Keyring
	keyInfo           keyring.Info
	ConnectedAddress  string
	ChainID           string
	BroadcastMode
	GasPrices string
}

// NewClient returns a new API client.
func NewClient(nodeURL string, armor string, passphrase string, chainID string, options *ClientOptions) (*Client, error) {
	if options == nil {
		options = &ClientOptions{}
	}
	if options.BroadcastMode == "" {
		options.BroadcastMode = BroadcastBlock
	}
	if options.GasPrices == nil {
		options.GasPrices = &DefaultGasPrice
	}

	SetPrefixes(prefix)
	newClient, err := client.NewClientFromNode(nodeURL)
	if err != nil {
		return nil, err
	}
	kr := keyring.NewInMemory(func(options *keyring.Options) {
		options.SupportedAlgos = keyring.SigningAlgoList{
			hd.Secp256k1,
		}
	})
	importAccountName := uuid.New().String()
	kr.ImportPrivKey(importAccountName, armor, passphrase)
	keyInfo, err := kr.Key(importAccountName)
	if err != nil {
		return nil, err
	}
	connectedAddress := sdk.AccAddress(keyInfo.GetPubKey().Address()).String()

	return &Client{
		nodeURL:           nodeURL,
		armor:             armor,
		passphrase:        passphrase,
		importAccountName: importAccountName,
		cosmosClient:      newClient,
		kr:                kr,
		keyInfo:           keyInfo,
		ConnectedAddress:  connectedAddress,
		BroadcastMode:     options.BroadcastMode,
		ChainID:           chainID,
		GasPrices:         *options.GasPrices,
	}, nil
}

func SetPrefixes(accountAddressPrefix string) {
	// Set prefixes
	accountPubKeyPrefix := accountAddressPrefix + "pub"
	validatorAddressPrefix := accountAddressPrefix + "valoper"
	validatorPubKeyPrefix := accountAddressPrefix + "valoperpub"
	consNodeAddressPrefix := accountAddressPrefix + "valcons"
	consNodePubKeyPrefix := accountAddressPrefix + "valconspub"

	// Set and seal config
	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount(accountAddressPrefix, accountPubKeyPrefix)
	config.SetBech32PrefixForValidator(validatorAddressPrefix, validatorPubKeyPrefix)
	config.SetBech32PrefixForConsensusNode(consNodeAddressPrefix, consNodePubKeyPrefix)
	config.Seal()
}

func (c *Client) GenerateOrBroadcastTx(msg ...sdk.Msg) (*sdk.TxResponse, error) {
	encodingConfig := simapp.MakeTestEncodingConfig()
	encodingConfig.InterfaceRegistry.RegisterImplementations((*sdk.Msg)(nil), &nftmngrtypes.MsgPerformActionByAdmin{})

	clientCtx := client.Context{}.
		WithBroadcastMode(string(c.BroadcastMode)).
		WithSkipConfirmation(true).
		WithClient(c.cosmosClient).
		WithKeyring(c.kr).
		WithAccountRetriever(types.AccountRetriever{}).
		WithFromName(c.importAccountName).
		WithFromAddress(sdk.AccAddress(c.keyInfo.GetPubKey().Address())).
		WithCodec(encodingConfig.Marshaler).
		WithInterfaceRegistry(encodingConfig.InterfaceRegistry).
		WithLegacyAmino(encodingConfig.Amino).
		WithChainID(c.ChainID).
		WithTxConfig(encodingConfig.TxConfig)

	flags := flag.NewFlagSet("client", flag.ExitOnError)

	fac := tx.NewFactoryCLI(clientCtx, flags).
		WithGas(0).
		WithSimulateAndExecute(true).
		WithGasAdjustment(1.5).
		WithGasPrices(c.GasPrices)

	txResponse, err := generateOrBroadcastTxWithFactory(clientCtx, fac, msg...)
	if err != nil {
		return nil, err
	}
	return txResponse, nil
}

func generateOrBroadcastTxWithFactory(clientCtx client.Context, txf tx.Factory, msgs ...sdk.Msg) (*sdk.TxResponse, error) {
	// Validate all msgs before generating or broadcasting the tx.
	// We were calling ValidateBasic separately in each CLI handler before.
	// Right now, we're factorizing that call inside this function.
	// ref: https://github.com/cosmos/cosmos-sdk/pull/9236#discussion_r623803504
	for _, msg := range msgs {
		if err := msg.ValidateBasic(); err != nil {
			return nil, err
		}
	}

	return broadcastTx(clientCtx, txf, msgs...)
}

func broadcastTx(clientCtx client.Context, txf tx.Factory, msgs ...sdk.Msg) (*sdk.TxResponse, error) {
	txf, err := prepareFactory(clientCtx, txf)
	if err != nil {
		return nil, err
	}

	if txf.SimulateAndExecute() || clientCtx.Simulate {
		_, adjusted, err := tx.CalculateGas(clientCtx, txf, msgs...)
		if err != nil {
			return nil, err
		}

		txf = txf.WithGas(adjusted)
	}

	if clientCtx.Simulate {
		return nil, nil
	}

	txn, err := tx.BuildUnsignedTx(txf, msgs...)
	if err != nil {
		return nil, err
	}

	txn.SetFeeGranter(clientCtx.GetFeeGranterAddress())
	err = tx.Sign(txf, clientCtx.GetFromName(), txn, true)
	if err != nil {
		return nil, err
	}

	txBytes, err := clientCtx.TxConfig.TxEncoder()(txn.GetTx())
	if err != nil {
		return nil, err
	}

	// broadcast to a Tendermint node
	res, err := clientCtx.BroadcastTx(txBytes)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func prepareFactory(clientCtx client.Context, txf tx.Factory) (tx.Factory, error) {
	from := clientCtx.GetFromAddress()

	if err := clientCtx.AccountRetriever.EnsureExists(clientCtx, from); err != nil {
		return txf, err
	}

	initNum, initSeq := txf.AccountNumber(), txf.Sequence()
	if initNum == 0 || initSeq == 0 {
		num, seq, err := clientCtx.AccountRetriever.GetAccountNumberSequence(clientCtx, from)
		if err != nil {
			return txf, err
		}

		if initNum == 0 {
			txf = txf.WithAccountNumber(num)
		}

		if initSeq == 0 {
			txf = txf.WithSequence(seq)
		}
	}

	return txf, nil
}

func (c *Client) QueryClient() nftmngrtypes.QueryClient {

	clientCtx := client.Context{}.
		WithClient(c.cosmosClient).
		WithKeyring(c.kr).
		WithAccountRetriever(types.AccountRetriever{})

	queryClient := nftmngrtypes.NewQueryClient(clientCtx)
	return queryClient
}
