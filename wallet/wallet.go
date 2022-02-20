package wallet

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcutil/coinset"
	"github.com/btcsuite/btcutil/hdkeychain"
	elemaddr "github.com/vulpemventures/go-elements/address"
	"github.com/vulpemventures/go-elements/elementsutil"
	"github.com/vulpemventures/go-elements/network"
	"github.com/vulpemventures/go-elements/payment"
	"github.com/vulpemventures/go-elements/transaction"
)

type AddressStats struct {
	Address string `json:"address""`
	ChainStats *AddressUtxoInfos `json:"chain_stats"`
	MempoolStats *AddressUtxoInfos `json:"mempool_stats"`
}

type AddressUtxoInfos struct {
	FundedTxoCount uint32 `json:"funded_txo_count"`
	SpentTxoCount uint32 `json:"spent_txo_count"`
	TxCount uint32 `json:"tx_count"`
}

type EsploraUtxo struct {
	TxId string `json:"txid"`
	Vout uint32 `json:"vout"`
	SatAmt uint64 `json:"Value"`
	Asset string `json:"Asset"`
	Status *Status `json:"Status"`
	Address string
}

func (e *EsploraUtxo) Hash() *chainhash.Hash {
	hash,_ := chainhash.NewHashFromStr(e.TxId)
	return hash
}

func (e *EsploraUtxo) Index() uint32 {
	return e.Vout
}

func (e *EsploraUtxo) Value() btcutil.Amount {
	//amt, _ := btcutil.NewAmount(float64(e.SatAmt))
	amt, _ := btcutil.NewAmount(float64(e.SatAmt))
	return amt
}

func (e *EsploraUtxo) PkScript() []byte {
	pkScript, _ := elemaddr.ToOutputScript(e.Address)
	return pkScript
}

func (e *EsploraUtxo) NumConfs() int64 {
	return 1
}

func (e *EsploraUtxo) ValueAge() int64 {
	return 0
}

func (e *EsploraUtxo) String() string {
	bytes,_ := json.Marshal(e)
	return string(bytes)
}
type Status struct {
	Confirmed bool `json:"confirmed"`
	BlockHeight uint32 `json:"block_height"`
	BlockHash string `json:"block_hash"`
	BlockTime uint64 `json:"block_time"`
}

type AddressInfo struct {
	Derivation uint32
	Address string
	Utxos []*EsploraUtxo
}

type EsploraApi interface {
	GetUtxosFromAddress(address string) ([]*EsploraUtxo, error)
	PostRawtransaction(rawTx string) (string, error)
	GetAddressStats(address string) (*AddressStats, error)
}


type LiquidWallet struct {
	esplora EsploraApi

	chaincfg *chaincfg.Params
	liquidNetwork *network.Network

	addressesWithUtxos []string

	unblindedAddrKey *hdkeychain.ExtendedKey

	blindedAddrKey *hdkeychain.ExtendedKey
	blinderKeys *hdkeychain.ExtendedKey

	addressToUtxoMap map[string]*AddressInfo
	utxos []*EsploraUtxo

	lbtcAsset []byte
}

func NewLiquidWallet(esplora EsploraApi, chaincfg *chaincfg.Params, liquidNetwork *network.Network) *LiquidWallet {
	return &LiquidWallet{esplora: esplora, chaincfg: chaincfg, liquidNetwork: liquidNetwork, lbtcAsset: append(
	[]byte{0x01},
		elementsutil.ReverseBytes(h2b(liquidNetwork.AssetID))...,
	)}
}

func (l *LiquidWallet) Initialize(seed []byte) error {
	baseKey, err := l.getBasekey(seed)
	if err != nil {
		return err
	}

	// Derive the extended key for the account unblinded addresses.  This gives
	// the path:
	//   m/0H/0
	unblindedAddrKey, err := baseKey.Derive(0)
	if err != nil {
		return err
	}
	l.unblindedAddrKey = unblindedAddrKey

	// Derive the extended key for the account unblinded addresses.  This gives
	// the path:
	//   m/0H/1
	blindedAddrKey, err := baseKey.Derive(1)
	if err != nil {
		return err
	}
	l.blindedAddrKey = blindedAddrKey

	// Derive the extended key for the account blinding keys.  This gives
	// the path:
	//   m/0H/2
	blinderKeys, err := baseKey.Derive(2)
	if err != nil {
		return err
	}
	l.blindedAddrKey = blinderKeys

	l.addressToUtxoMap = make(map[string]*AddressInfo)
	l.utxos = []*EsploraUtxo{}

	err = l.resetUtxos()
	if err != nil {
		return err
	}

	return nil
}

// getBasekey returns the basekey for the wallet from a given bit39 seed
func (l *LiquidWallet) getBasekey(seed []byte) (*hdkeychain.ExtendedKey, error) {
	// Generate a new master node using the seed.
	masterKey, err := hdkeychain.NewMaster(seed, l.chaincfg)
	if err != nil {
		return nil, err
	}

	// Derive the extended key for account 0.  This gives the path:
	//   m/0H
	acct0, err := masterKey.Derive(hdkeychain.HardenedKeyStart + 0)
	if err != nil {
		return nil, err
	}

	return acct0, nil
}

// getSpendAddresses returns all addresses with utxos on them, as well as the current address with no funds
func (l *LiquidWallet) getSpendAddresses(depth int) (map[string]*AddressInfo,[]*EsploraUtxo, error) {
	startingDerivation := uint32(0)
	addrToInfoMap := make(map[string]*AddressInfo)
	var allUtxos []*EsploraUtxo
	for i := 0; i < depth; i++  {
		addr, err := l.getUnblindedAddressFromKey(l.unblindedAddrKey, startingDerivation)
		if err != nil {
			return nil, nil,err
		}
		utxos, err :=l.esplora.GetUtxosFromAddress(addr)
		if err != nil {
			return nil, nil, err
		}
		addrToInfoMap[addr] = &AddressInfo{
			Derivation: startingDerivation,
			Address: addr,
			Utxos: utxos,
		}
		allUtxos = append(allUtxos, utxos...)
		startingDerivation++
	}
	return addrToInfoMap, allUtxos,  nil
}


func (l *LiquidWallet) getUnblindedAddressFromKey(key *hdkeychain.ExtendedKey, i uint32) (string,error) {
	acct, err := key.Derive(i)
	if err != nil {
		return "", err
	}
	addrPrivkey,err := acct.ECPrivKey()
	if err != nil {
		return "", err
	}

	addrPubkey := addrPrivkey.PubKey()

	p2wpkh := payment.FromPublicKey(addrPubkey, l.liquidNetwork, nil)
	addr,err := p2wpkh.WitnessPubKeyHash()
	if err != nil {
		return "", err
	}
	return addr, nil
}


func (l *LiquidWallet) updateUtxoMap() error {
	addressesWithUtxos,utxos, err := l.getSpendAddresses(200)
	if err != nil {
		return err
	}
	l.utxos = append(l.utxos, utxos...)
	for k,v := range addressesWithUtxos {
		l.addressToUtxoMap[k] = v
	}
	return nil
}

func (l *LiquidWallet) resetUtxos() error {
	addressesWithUtxos,utxos, err := l.getSpendAddresses(200)
	if err != nil {
		return err
	}
	l.addressToUtxoMap = addressesWithUtxos
	l.utxos = utxos
	return nil
}

func (l *LiquidWallet) GetUtxos() ([]*EsploraUtxo, error) {
	_,utxos, err := l.getSpendAddresses(200)
	if err != nil {
		return nil, err
	}
	return utxos, nil
}

func (l *LiquidWallet) GetAddress() (string, error) {
	return l.getFirstUnusedAddress(200, 0)
}


func (l *LiquidWallet) getFirstUnusedAddress(maxDepth int, startingDepth int) (string, error) {
	// fixme this seems weird
	if maxDepth >= 100000 {
		return "", errors.New("max depth seems excessively large")
	}

	if startingDepth > maxDepth {
		return "", errors.New("maxDepth must be larger than startingDepth")
	}

	unusedAddr := ""
	for i := startingDepth;i < maxDepth;i++ {
		addr, err := l.getUnblindedAddressFromKey(l.unblindedAddrKey, uint32(i))
		if err != nil {
			return "",err
		}
		addrStats, err := l.esplora.GetAddressStats(addr)
		if err != nil {
			return "",err
		}
		if addrStats.ChainStats.TxCount == 0 && addrStats.MempoolStats.TxCount == 0 {
			unusedAddr = addrStats.Address
			break
		}
	}
	if unusedAddr == "" {
		return l.getFirstUnusedAddress(maxDepth + 100, startingDepth+maxDepth)
	}
	return unusedAddr, nil

}

func (l *LiquidWallet) SendToAddress(address string, asset string, value uint64) (string, error) {
	inputs, totalInputValue, err := l.GetInputs(asset, value)
	if err != nil {
		return "", err
	}

	var txInputs []*transaction.TxInput
	var inputSigner []*hdkeychain.ExtendedKey
	for _,v := range inputs {
		txInputs = append(txInputs, EsploraUtxoToTxInput(v))
		key, err := l.unblindedAddrKey.Derive(l.addressToUtxoMap[v.Address].Derivation)
		if err != nil {
			return "", err
		}
		//privkey, err := key.ECPrivKey()
		//if err != nil {
		//	return "", err
		//}

		inputSigner = append(inputSigner, key)
	}
	// todo get esplora fee
	fee := uint64(500)
	changeValue := totalInputValue - value - fee

	receiverOutputScript,err := elemaddr.ToOutputScript(address)
	if err != nil {
		return "", err
	}
	receiverValue, _ := elementsutil.SatoshiToElementsValue(value)
	receiverScript := receiverOutputScript
	receiverOutput := transaction.NewTxOutput(l.lbtcAsset, receiverValue, receiverScript)

	nextAddr, err := l.GetAddress()
	if err != nil {
		return "", err
	}

	feeValue, _ := elementsutil.SatoshiToElementsValue(fee)
	feeScript := []byte{}
	feeOutput := transaction.NewTxOutput(l.lbtcAsset, feeValue, feeScript)

	outputs := []*transaction.TxOutput{receiverOutput, feeOutput}

	if changeValue > 0 {
		changeOutputScript, err := elemaddr.ToOutputScript(nextAddr)
		if err != nil {
			return "", err
		}
		changeValueBytes, _ := elementsutil.SatoshiToElementsValue(changeValue)
		changeScript := changeOutputScript
		changeOutput := transaction.NewTxOutput(l.lbtcAsset, changeValueBytes, changeScript)
		outputs = append(outputs, changeOutput)
	}

	tx := transaction.NewTx(2)

	tx.Inputs = txInputs
	tx.Outputs = outputs

	//pset, err := pset2.New(txInputs, outputs, 2,0)
	//if err != nil {
	//	return "", err
	//}
	//updater, err := pset2.NewUpdater(pset)
	//if err != nil {
	//	return "", err
	//}
	//var sigs [][]byte
	for i, privkey := range inputSigner {
		//if err := updater.AddInSighashType(txscript.SigHashAll, i); err != nil {
		//	return "", err
		//}
		outputScript, err := elemaddr.ToOutputScript(inputs[i].Address)
		if err!= nil {
			return "",err
		}
		p2wpkhPayment, err := payment.FromScript(outputScript, l.liquidNetwork,nil)
		if err!= nil {
			return "",err
		}
		if elemaddr.GetScriptType(outputScript) != elemaddr.P2WpkhScript {
			return "", errors.New("input should be p2wpkh")
		}
		inputValue,_ := elementsutil.SatoshiToElementsValue(inputs[i].SatAmt)
		sigHash := tx.HashForWitnessV0(i,p2wpkhPayment.Script, inputValue, txscript.SigHashAll)
		signer, err := privkey.ECPrivKey()
		if err != nil {
			return "", err
		}
		signature, err := signer.Sign(sigHash[:])
		if err!= nil {
			return "",err
		}
		sigWithHashType := append(signature.Serialize(), byte(txscript.SigHashAll))

		pubkey, err := privkey.ECPubKey()
		if err!= nil {
			return "",err
		}
		tx.Inputs[i].Witness = [][]byte{ sigWithHashType,pubkey.SerializeCompressed()}
	}
	txString, err := tx.ToHex()
	if err != nil {
		return "", err
	}

	txId, err := l.esplora.PostRawtransaction(txString)
	if err != nil {
		return "", err
	}

	return txId,nil
}


type Signer interface {
	Sign(hash []byte) (*btcec.Signature, error)
	PubKey() (*btcec.PublicKey)
}
func EsploraUtxoToTxInput(esploraUtxo *EsploraUtxo) (*transaction.TxInput) {
	txInputHash := elementsutil.ReverseBytes(h2b(esploraUtxo.TxId))
	txInputIndex := esploraUtxo.Vout
	txInput := transaction.NewTxInput(txInputHash, txInputIndex)
	return txInput
}

func (l *LiquidWallet) GetInputs(asset string, value uint64) ([]*EsploraUtxo,uint64, error) {
	selector := &coinset.MaxValueAgeCoinSelector{
		MaxInputs:       10,
		MinChangeAmount: 5000,
	}
	var coins []coinset.Coin
	for _,v := range l.utxos {
		if paymentType,err := GetPaymentType(v.Address); err != nil || paymentType != elemaddr.P2WpkhScript {
			continue
		}
		coins = append(coins, v)
	}
	amt,_ := btcutil.NewAmount(float64(value+2000))
	coinSet, err := selector.CoinSelect(amt, coins)
	if err != nil {
		return nil,0, err
	}
	var totalValue uint64
	var utxos []*EsploraUtxo
	for _,v := range coinSet.Coins() {
		utxos = append(utxos, v.(*EsploraUtxo))
		totalValue += uint64(v.Value().ToBTC())
	}
	return  utxos,totalValue, nil
}

func GetPaymentType(address string ) (int, error) {
	outputScript, err := elemaddr.ToOutputScript(address)
	if err != nil {
		return 0, err
	}
	return elemaddr.GetScriptType(outputScript), nil
}



func b2h(buf []byte) string {
	return hex.EncodeToString(buf)
}

func h2b(str string) []byte {
	buf, _ := hex.DecodeString(str)
	return buf
}