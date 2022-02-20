package chain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/sputn1ck/liquid-go-lightwallet/wallet"
	"io/ioutil"
	"log"
	"net/http"
)

type EsploraApi struct {
	baseUrl string

	client *http.Client
}

func NewEsploraApi(baseUrl string) *EsploraApi {
	return &EsploraApi{baseUrl: baseUrl, client: &http.Client{}}
}

func (e *EsploraApi) GetUtxosFromAddress(address string) ([]*wallet.EsploraUtxo, error) {
	resp, err := e.client.Get(fmt.Sprintf("%s/address/%s/utxo", e.baseUrl, address))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var utxos []*wallet.EsploraUtxo
	err = json.Unmarshal(bodyBytes, &utxos)
	if err != nil {
		return nil, err
	}
	for _,v := range utxos {
		v.Address = address
	}
	return utxos,nil
}


func (e *EsploraApi) PostRawtransaction(rawTx string) (string, error) {
	resp, err := e.client.Post(fmt.Sprintf("%s/tx", e.baseUrl), "string",bytes.NewBuffer([]byte(rawTx)))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	log.Printf("%s", bodyBytes)
	return string(bodyBytes), nil
}

func (e *EsploraApi) GetAddressStats(address string) (*wallet.AddressStats, error) {
	resp, err := e.client.Get(fmt.Sprintf("%s/address/%s", e.baseUrl, address))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var addrInfo *wallet.AddressStats
	err = json.Unmarshal(bodyBytes, &addrInfo)
	if err != nil {
		return nil, err
	}
	return addrInfo,nil
}
