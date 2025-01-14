package go_eth_multicall

import (
	"context"
	"encoding/json"
	"math/big"
	"strings"

	MultiCall2 "github.com/bezier-fi/go-eth-multicall/contracts/MultiCall"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

type Call struct {
	Name     string         `json:"name"`
	Target   common.Address `json:"target"`
	CallData []byte         `json:"call_data"`
}

type CallResponse struct {
	Success    bool   `json:"success"`
	ReturnData []byte `json:"returnData"`
}

func (call Call) GetMultiCall() MultiCall2.Multicall2Call {
	return MultiCall2.Multicall2Call{Target: call.Target, CallData: call.CallData}
}

var contracts = map[int64]string{
	1:     "0x5BA1e12693Dc8F9c48aAD8770482f4739bEeD696",
	137:   "0xed386Fe855C1EFf2f843B910923Dd8846E45C5A4",
	43114: "0x29b6603d17b9d8f021ecb8845b6fd06e1adf89de",
}

func randomSigner() *bind.TransactOpts {
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		panic(err)
	}

	signer, err := bind.NewKeyedTransactorWithChainID(privateKey, big.NewInt(1))
	if err != nil {
		panic(err)
	}

	signer.NoSend = true
	signer.Context = context.Background()
	signer.GasPrice = big.NewInt(0)

	return signer
}

type EthMultiCaller struct {
	Signer          *bind.TransactOpts
	Client          *ethclient.Client
	Abi             abi.ABI
	ContractAddress common.Address
}

func New(rawurl string) EthMultiCaller {
	client, err := ethclient.Dial(rawurl)
	if err != nil {
		panic(err)
	}

	// Load Multicall abi for later use
	mcAbi, err := abi.JSON(strings.NewReader(MultiCall2.MultiCall2ABI))
	if err != nil {
		panic(err)
	}

	chainID, err := client.ChainID(context.Background())
	if err != nil {
		panic(err)
	}

	contractAddress := common.HexToAddress(contracts[chainID.Int64()])

	return EthMultiCaller{
		Signer:          randomSigner(),
		Client:          client,
		Abi:             mcAbi,
		ContractAddress: contractAddress,
	}
}

func (caller *EthMultiCaller) Execute(calls []Call, blockNumber *big.Int) map[string]CallResponse {
	var responses []CallResponse

	var multiCalls = make([]MultiCall2.Multicall2Call, 0, len(calls))

	// Add calls to multicall structure for the contract
	for _, call := range calls {
		multiCalls = append(multiCalls, call.GetMultiCall())
	}

	// Prepare calldata for multicall
	callData, err := caller.Abi.Pack("tryAggregate", false, multiCalls)
	if err != nil {
		panic(err)
	}

	// Perform multicall
	resp, err := caller.Client.CallContract(context.Background(), ethereum.CallMsg{To: &caller.ContractAddress, Data: callData}, blockNumber)
	if err != nil {
		panic(err)
	}

	// Unpack results
	unpackedResp, _ := caller.Abi.Unpack("tryAggregate", resp)

	a, err := json.Marshal(unpackedResp[0])
	if err != nil {
		panic(err)
	}

	err = json.Unmarshal(a, &responses)
	if err != nil {
		panic(err)
	}

	// Create mapping for results. Be aware that we sometimes get two empty results initially, unsure why
	results := make(map[string]CallResponse)
	for i, response := range responses {
		results[calls[i].Name] = response
	}

	return results
}
