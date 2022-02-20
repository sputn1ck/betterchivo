package main

import (
	"fmt"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/sputn1ck/liquid-go-lightwallet/chain"
	"github.com/sputn1ck/liquid-go-lightwallet/wallet"
	"github.com/tyler-smith/go-bip39"
	"github.com/vulpemventures/go-elements/network"
	"log"
)

var (
	liquidNetwork = &network.Regtest
)

func main() {
	if err := run(); err != nil {
		log.Printf("Error: %v", err)
	}
}

var (
	accounts = []string{
		"alter element derive simple area banana anxiety kiss action digital prevent people volume gaze public divorce box lock issue aspect shadow program furnace husband",
		"mesh utility girl umbrella brief fatigue hamster garage egg olympic party keen vacant nation coconut run multiply talk supply muffin trumpet immune bunker envelope",
		"blossom must cherry inform whale steak wish raw arm among run dog middle animal horse history sustain extra trend walnut orchard grass bid caution",
		"veteran buzz mammal found sign sick steel butter message usage middle easy",
	}
)



func run() error {
	seed := bip39.NewSeed(accounts[0], "")

	cfgCfgParams := setChainCfgParams(liquidNetwork, &chaincfg.MainNetParams)

	esplora := chain.NewEsploraApi("http://localhost:3001")

	wallet := wallet.NewLiquidWallet(esplora, cfgCfgParams, liquidNetwork)
	err := wallet.Initialize(seed)
	if err != nil {
		return err
	}
	lastAddr, err := wallet.GetAddress()
	if err != nil {
		return err
	}
	fmt.Printf("next address %s \n", lastAddr)

	//inputs, totalValue, err := wallet.GetInputs("", 50000)
	//if err != nil {
	//	return err
	//}
	//log.Printf("inputs: totalValue %v addr[0]: %v \n", totalValue, inputs[0].Address)
	utxos, err := wallet.GetUtxos()
	for _, v:= range utxos {
		log.Printf("%s", v)
	}
	res, err := wallet.SendToAddress(lastAddr,"",50000)
	if err != nil {
		return err
	}
	log.Printf("%s", res)

	return nil
}






func setChainCfgParams(liquidNetwork *network.Network, chaincfg *chaincfg.Params) *chaincfg.Params {
	chaincfg.Name = liquidNetwork.Name
	chaincfg.Bech32HRPSegwit = liquidNetwork.Bech32
	chaincfg.HDPrivateKeyID = liquidNetwork.HDPrivateKey
	chaincfg.HDPublicKeyID = liquidNetwork.HDPublicKey
	chaincfg.PubKeyHashAddrID = liquidNetwork.PubKeyHash
	chaincfg.ScriptHashAddrID = liquidNetwork.ScriptHash
	chaincfg.PrivateKeyID = liquidNetwork.Wif
	return chaincfg
}
