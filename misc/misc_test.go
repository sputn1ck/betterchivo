package misc

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/txscript"
	"github.com/vulpemventures/go-elements/elementsutil"
	"github.com/vulpemventures/go-elements/network"
	"github.com/vulpemventures/go-elements/payment"
	"github.com/vulpemventures/go-elements/transaction"
	"github.com/ybbus/jsonrpc"
	"io/ioutil"
	"log"
	"net/http"
	"testing"
	"time"
)
var lbtc = append(
	[]byte{0x01},
	elementsutil.ReverseBytes(h2b(network.Regtest.AssetID))...,
)

func Test_AssetFillOut(t *testing.T) {
	// Generating Alices Keys and Address
	privkeyAlice, err := btcec.NewPrivateKey(btcec.S256())
	if err != nil {
		t.Fatal(err)
	}
	pubkeyAlice := privkeyAlice.PubKey()
	p2wpkhAlice := payment.FromPublicKey(pubkeyAlice, &network.Regtest, nil)
	addressAlice, _ := p2wpkhAlice.WitnessPubKeyHash()

	//
	privkeyBob, err := btcec.NewPrivateKey(btcec.S256())
	if err != nil {
		t.Fatal(err)
	}
	pubkeyBob := privkeyBob.PubKey()
	p2wpkhBob := payment.FromPublicKey(pubkeyBob, &network.Regtest, nil)
	addressBob, _ := p2wpkhBob.WitnessPubKeyHash()

	// Generating Charlies Keys and Address
	privkeyCharlie, err := btcec.NewPrivateKey(btcec.S256())
	if err != nil {
		t.Fatal(err)
	}
	pubkeyCharlie := privkeyCharlie.PubKey()
	p2wpkhCharlie := payment.FromPublicKey(pubkeyCharlie, &network.Regtest, nil)
	//addressCharlie, _ := p2wpkhCharlie.WitnessPubKeyHash()


	// Fund Alice address with LBTC.
	if _, err := faucet(addressAlice); err != nil {
		t.Fatal(err)
	}

	// Fund Bob address with an asset.
	_, mintedAsset, err := mint(addressBob, 1, "VULPEM", "VLP")
	if err != nil {
		t.Fatal(err)
	}
	err = generate(1)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Second)

	// Retrieve Alice utxos.
	utxosAlice, err := unspents(addressAlice)
	if err != nil {
		t.Fatal(err)
	}

	// Retrieve Bob utxos.
	utxosBob, err := unspents(addressBob)
	if err != nil {
		t.Fatal(err)
	}
	log.Printf("%s %s", addressBob, addressAlice)
	log.Printf("%v %v", utxosAlice, utxosBob)

	// The transaction will have 2 input and 3 outputs.

	// Input From Alice
	valueAliceRaw := utxosAlice[0]["value"].(float64)
	valueAlice := uint64(valueAliceRaw)
	valueBytesAlice,err := elementsutil.SatoshiToElementsValue(valueAlice)
	if err != nil {
		t.Fatal(err)
	}
	txInputHashAlice := elementsutil.ReverseBytes(h2b(utxosAlice[0]["txid"].(string)))
	txInputIndexAlice := uint32(utxosAlice[0]["vout"].(float64))
	txInputAlice := transaction.NewTxInput(txInputHashAlice, txInputIndexAlice)

	// Input From Bob

	valueBob := uint64(utxosBob[0]["value"].(float64))
	valueBytesBob,err := elementsutil.SatoshiToElementsValue(valueBob)
	if err != nil {
		t.Fatal(err)
	}
	txInputHashBob := elementsutil.ReverseBytes(h2b(utxosBob[0]["txid"].(string)))
	txInputIndexBob := uint32(utxosBob[0]["vout"].(float64))
	txInputBob := transaction.NewTxInput(txInputHashBob, txInputIndexBob)

	//// Outputs from Alice

	fee := uint64(500)
	// Fee Lbtc
	feeValue, _ := elementsutil.SatoshiToElementsValue(fee)
	feeScript := []byte{}
	feeOutput := transaction.NewTxOutput(lbtc, feeValue, feeScript)

	// Change from/to Alice
	changeScriptAlice := p2wpkhAlice.WitnessScript
	changeValueAlice, _ := elementsutil.SatoshiToElementsValue(uint64(valueAlice - fee))
	changeOutputAlice := transaction.NewTxOutput(lbtc, changeValueAlice[:], changeScriptAlice)

	// Asset hex
	asset := append([]byte{0x01}, elementsutil.ReverseBytes(h2b(mintedAsset))...)

	// Asset to Charlie
	bobToCharlieValue, _ := elementsutil.SatoshiToElementsValue(valueBob)
	bobToCharlieScript := p2wpkhCharlie.WitnessScript
	bobToCharlieOutput := transaction.NewTxOutput(asset, bobToCharlieValue, bobToCharlieScript)

	inputs := []*transaction.TxInput{txInputAlice, txInputBob}
	outputs := []*transaction.TxOutput{bobToCharlieOutput, changeOutputAlice, feeOutput}

	tx := transaction.NewTx(2)
	tx.Inputs = inputs
	tx.Outputs = outputs

	err = signInputs(tx, []*btcec.PrivateKey{privkeyAlice, privkeyBob}, [][]byte{p2wpkhAlice.Script, p2wpkhBob.Script}, [][]byte{valueBytesAlice, valueBytesBob})
	if err != nil {
		t.Fatal(err)
	}

	txHex, err := tx.ToHex()
	if err != nil {
		t.Fatal(err)
	}
	txId, err := broadcast(txHex)
	if err !=nil {
		t.Fatal(err)
	}
	log.Printf("test successfull %s", txId)

}

func signInputs(tx *transaction.Transaction, signers []*btcec.PrivateKey, scripts [][]byte, values[][]byte) error {
	for i,v := range tx.Inputs {
		sigHash := tx.HashForWitnessV0(i, scripts[i], values[i], txscript.SigHashAll)

		signature, err := signers[i].Sign(sigHash[:])
		if err != nil {
			return err
		}
		sigWithHashType := append(signature.Serialize(), byte(txscript.SigHashAll))
		pubkey  := signers[i].PubKey().SerializeCompressed()
		v.Witness = [][]byte{ sigWithHashType[:], pubkey[:]}
	}
	return nil
}

func unspents(address string) ([]map[string]interface{}, error) {
	getUtxos := func(address string) ([]interface{}, error) {
		baseUrl, err := apiBaseUrl()
		if err != nil {
			return nil, err
		}
		url := fmt.Sprintf("%s/address/%s/utxo", baseUrl, address)
		resp, err := http.Get(url)
		if err != nil {
			return nil, err
		}
		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		var respBody interface{}
		if err := json.Unmarshal(data, &respBody); err != nil {
			return nil, err
		}
		return respBody.([]interface{}), nil
	}

	utxos := []map[string]interface{}{}
	for len(utxos) <= 0 {
		time.Sleep(1 * time.Second)
		u, err := getUtxos(address)
		if err != nil {
			return nil, err
		}
		for _, unspent := range u {
			utxo := unspent.(map[string]interface{})
			utxos = append(utxos, utxo)
		}
	}

	return utxos, nil
}


func faucet(address string) (string, error) {
	liquidrpc := getLiquidRpc("")

	res, err := liquidrpc.Call("sendtoaddress", address, 1)

	if err != nil {
		return "", err
	}
	if res.Error != nil {
		return "",res.Error
	}
	return res.GetString()
}

func generate(amount int) (error) {
	liquidrpc := getLiquidRpc("")

	res, err := liquidrpc.Call("generatetoaddress", amount, "ert1qfkht0df45q00kzyayagw6vqhfhe8ve7z7wecm0xsrkgmyulewlzqumq3ep")
	if err != nil {
		return err
	}
	if res.Error != nil {
		return res.Error
	}
	return nil
}

func broadcast(txHex string) (string , error) {
	liquidrpc := getLiquidRpc("")

	res, err := liquidrpc.Call("sendrawtransaction", txHex)
	if err != nil {
		return "",err
	}
	if res.Error != nil {
		return "",res.Error
	}
	return res.GetString()
}



func mint(address string, quantity int, name string, ticker string) (string, string, error) {
	liquidrpc := getLiquidRpc("")

	res, err := liquidrpc.Call("issueasset", quantity, 0, false)
	if err != nil {
		return "", "", err
	}
	if res.Error != nil {
		return "", "", res.Error
	}
	decodedRes := res.Result.(map[string]interface{})
	asset := decodedRes["asset"].(string)

	res, err = liquidrpc.Call("sendtoaddress", address, quantity, "","", false, false, 1, "UNSET",  asset,true)
	if err != nil {
		return "", "", err
	}
	if res.Error != nil {
		return "", "", res.Error
	}
	txId, err := res.GetString()
	if err != nil {
		return "", "", err
	}
	return txId,asset,nil
}

func getLiquidRpc(wallet string) jsonrpc.RPCClient {
	authPair := fmt.Sprintf("%s:%s", "admin1", "123")
	authPairb64 := base64.RawURLEncoding.EncodeToString([]byte(authPair))
	authHeader := []byte("Basic ")
	authHeader = append(authHeader, []byte(authPairb64)...)
	addr := fmt.Sprintf("http://127.0.0.1:18884/wallet/%s", wallet)

	return jsonrpc.NewClientWithOpts(addr,&jsonrpc.RPCClientOpts{
		CustomHeaders: map[string]string{
			"Authorization": string(authHeader),
		},
	})
}

func apiBaseUrl() (string, error) {
	return "http://localhost:3001",nil
}
func b2h(buf []byte) string {
	return hex.EncodeToString(buf)
}

func h2b(str string) []byte {
	buf, _ := hex.DecodeString(str)
	return buf
}