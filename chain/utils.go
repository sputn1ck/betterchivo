package chain

import (
	"encoding/hex"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/vulpemventures/go-elements/network"
)


func GetUsdtAsset(network string) string {
	if network == "testnet" {
		return "38fca2d939696061a8f76d4e6b5eecd54e3b4221c846f24a6b279e79952850a5"
	} else if network == "liquid" {
		return "ce091c998b83c78bb71a632313ba3760f1763d9cfcffae02258ffa9865a37bd2"
	}
	return ""
}


func GetChainCfgParams(liquidNetwork *network.Network, chaincfg *chaincfg.Params) *chaincfg.Params {
	chaincfg.Name = liquidNetwork.Name
	chaincfg.Bech32HRPSegwit = liquidNetwork.Bech32
	chaincfg.HDPrivateKeyID = liquidNetwork.HDPrivateKey
	chaincfg.HDPublicKeyID = liquidNetwork.HDPublicKey
	chaincfg.PubKeyHashAddrID = liquidNetwork.PubKeyHash
	chaincfg.ScriptHashAddrID = liquidNetwork.ScriptHash
	chaincfg.PrivateKeyID = liquidNetwork.Wif
	return chaincfg
}

// GetOpeningTxScript returns the script for the opening transaction of a swap,
// where the taker is the peer paying the invoice and the maker the peer providing the lbtc
func GetSwapScript(takerPubkeyHash []byte, makerPubkeyHash []byte, pHash []byte, csv uint32) ([]byte, error) {
	script := txscript.NewScriptBuilder().
		AddData(makerPubkeyHash).
		AddOp(txscript.OP_CHECKSIG).
		AddOp(txscript.OP_NOTIF).
		AddData(makerPubkeyHash).
		AddOp(txscript.OP_CHECKSIG).
		AddOp(txscript.OP_NOTIF).
		AddOp(txscript.OP_SIZE).
		AddData(h2b("20")).
		AddOp(txscript.OP_EQUALVERIFY).
		AddOp(txscript.OP_SHA256).
		AddData(pHash[:]).
		AddOp(txscript.OP_EQUALVERIFY).
		AddOp(txscript.OP_ENDIF).
		AddData(takerPubkeyHash).
		AddOp(txscript.OP_CHECKSIG).
		AddOp(txscript.OP_ELSE).
		AddInt64(int64(csv)).
		AddOp(txscript.OP_CHECKSEQUENCEVERIFY).
		AddOp(txscript.OP_ENDIF)
	return script.Script()
}

func h2b(str string) []byte {
	buf, _ := hex.DecodeString(str)
	return buf
}


