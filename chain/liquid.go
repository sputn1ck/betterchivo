package chain

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/txscript"
	"github.com/vulpemventures/go-elements/address"
	"github.com/vulpemventures/go-elements/elementsutil"
	"github.com/vulpemventures/go-elements/network"
	"github.com/vulpemventures/go-elements/payment"
	"github.com/vulpemventures/go-elements/transaction"
	"log"
)

type LiquidOnchain struct {
	network *network.Network
}

func NewLiquidOnchain(network *network.Network) *LiquidOnchain {
	return &LiquidOnchain{network: network}
}

func (l *LiquidOnchain) GetAsset() []byte {
	return append(
		[]byte{0x01},
		elementsutil.ReverseBytes(h2b(l.network.AssetID))...,
	)
}

func (l *LiquidOnchain) TranslateAsset(asset []byte) []byte{
	return append(
		[]byte{0x01},
		elementsutil.ReverseBytes(asset)...,
	)
}

type SwapOpeningParams struct {
	makerPubkey []byte
	takerPubkey []byte
	csv uint32
	phash []byte
	scriptOutputs []AssetAmountTuple
}

func NewSwapOpeningParams(makerPubkey []byte, takerPubkey []byte, csv uint32, phash []byte, scriptOutputs []AssetAmountTuple) SwapOpeningParams {
	return SwapOpeningParams{makerPubkey: makerPubkey, takerPubkey: takerPubkey, csv: csv, phash: phash, scriptOutputs: scriptOutputs}
}

type AssetAmountTuple struct {
	Asset  []byte
	Amount uint64
}

func (s *SwapOpeningParams) ToTxScript() ([]byte, error) {
	return GetOpeningTxScript(s.takerPubkey,s.makerPubkey, s.phash, s.csv)
}


func (l *LiquidOnchain) CreateUnfundedOpeningTransaction(params SwapOpeningParams) (string, error) {
	redeemScript, err := params.ToTxScript()
	if err != nil {
		return "", err
	}
	log.Printf(" redeem script %x", redeemScript)
	scriptPubKey := []byte{0x00, 0x20}
	witnessProgram := sha256.Sum256(redeemScript)
	scriptPubKey = append(scriptPubKey, witnessProgram[:]...)

	redeemPayment, _ := payment.FromScript(scriptPubKey, l.network, nil)

	scriptAddr, err := redeemPayment.WitnessScriptHash()
	if err != nil {
		return "", err
	}

	outputscript, err := address.ToOutputScript(scriptAddr)
	if err != nil {
		return "", err
	}

	tx := transaction.NewTx(2)

	for _,v := range params.scriptOutputs {
		sats, err := elementsutil.SatoshiToElementsValue(v.Amount)
		if err != nil {
			return "", err
		}
		output := transaction.NewTxOutput(v.Asset, sats, outputscript)
		tx.Outputs = append(tx.Outputs, output)
	}


	unfundedTxHex, err := tx.ToHex()
	if err != nil {
		return "", err
	}
	return unfundedTxHex, nil
}

type ClaimParams struct {
	openingTxHex string
	redeemAddress string
	assetAmount uint64
	csv uint32
	makerPubkey []byte
	takerPubkey []byte
	preimage []byte
	paymenthash []byte
	asset []byte
	signingKey *btcec.PrivateKey
}

func NewClaimParams(openingTxHex string, redeemAddress string, assetAmount uint64, csv uint32, makerPubkey []byte, takerPubkey []byte, preimage []byte, paymenthash []byte, asset []byte, signingKey *btcec.PrivateKey) ClaimParams {
	return ClaimParams{openingTxHex: openingTxHex, redeemAddress: redeemAddress, assetAmount: assetAmount, csv: csv, makerPubkey: makerPubkey, takerPubkey: takerPubkey, preimage: preimage, paymenthash: paymenthash, asset: asset, signingKey: signingKey}
}

func (l *LiquidOnchain) CreatePreimageSpendingTransaction(params ClaimParams) (string, error) {
	firstTx, err := transaction.NewTxFromHex(params.openingTxHex)
	if err != nil {
		return "", err
	}

	// get script vouts
	redeemScript, err := GetOpeningTxScript(params.takerPubkey, params.makerPubkey, params.paymenthash, params.csv)
	if err != nil {
		return "", err
	}
	log.Printf("redeem script %x", redeemScript)

	feeVout, err := l.FindVout(firstTx.Outputs, redeemScript, l.GetAsset())
	if err != nil {
		return "", err
	}

	assetVout, err := l.FindVout(firstTx.Outputs, redeemScript, params.asset)
	if err != nil {
		return "", err
	}

	// create new transaction
	spendingTx := transaction.NewTx(2)

	txHash := firstTx.TxHash()

	// add inputs
	feeInput := transaction.NewTxInput(txHash[:], feeVout)
	feeInput.Sequence = 0

	assetInput := transaction.NewTxInput(txHash[:], assetVout)
	assetInput.Sequence = 0

	feeOutputInIndex := 0
	assetOutputInIndex := 1

	spendingTx.Inputs = make([]*transaction.TxInput, 2)
	spendingTx.Inputs[feeOutputInIndex] = feeInput
	spendingTx.Inputs[assetOutputInIndex] = assetInput


	outputScript, err := address.ToOutputScript(params.redeemAddress)
	if err != nil {
		return "", err
	}


	feeOutput := transaction.NewTxOutput(l.GetAsset(), firstTx.Outputs[feeVout].Value, []byte{})
	assetOutput := transaction.NewTxOutput(params.asset, firstTx.Outputs[assetVout].Value, outputScript)

	spendingTx.Outputs = make([]*transaction.TxOutput,2)
	spendingTx.Outputs[feeOutputInIndex] = feeOutput
	spendingTx.Outputs[assetOutputInIndex] = assetOutput

	// create sigs and witnesses
	assetSighash := spendingTx.HashForWitnessV0(assetOutputInIndex, redeemScript[:], firstTx.Outputs[assetVout].Value, txscript.SigHashAll)
	feeSighash := spendingTx.HashForWitnessV0(feeOutputInIndex, redeemScript[:], firstTx.Outputs[feeVout].Value, txscript.SigHashAll)

	assetSig, err := params.signingKey.Sign(assetSighash[:])
	if err != nil {
		return "", err
	}

	feeSig, err := params.signingKey.Sign(feeSighash[:])
	if err != nil {
		return "", err
	}


	spendingTx.Inputs[assetOutputInIndex].Witness = GetPreimageWitness(assetSig.Serialize(), params.preimage, redeemScript)
	spendingTx.Inputs[feeOutputInIndex].Witness = GetPreimageWitness(feeSig.Serialize(), params.preimage, redeemScript)

	txHex, err := spendingTx.ToHex()
	if err != nil {
		return "", err
	}
	return txHex, nil
}

func (l *LiquidOnchain) FindVout(outputs []*transaction.TxOutput, redeemScript []byte, asset []byte) (uint32, error) {
	wantAddr, err := l.CreateOpeningAddress(redeemScript)
	if err != nil {
		return 0, err
	}
	wantBytes, err := address.ToOutputScript(wantAddr)
	if err != nil {
		return 0, err
	}

	for i, v := range outputs {
		if bytes.Compare(v.Script, wantBytes) == 0 && bytes.Compare(v.Asset, asset) == 0 {
			return uint32(i), nil
		}
	}
	return 0, errors.New("vout not found")
}

// creatOpeningAddress returns the address for the opening tx
func (l *LiquidOnchain) CreateOpeningAddress(redeemScript []byte) (string, error) {
	scriptPubKey := []byte{0x00, 0x20}
	witnessProgram := sha256.Sum256(redeemScript)
	scriptPubKey = append(scriptPubKey, witnessProgram[:]...)

	redeemPayment, err := payment.FromScript(scriptPubKey, l.network, nil)
	if err != nil {
		return "", err
	}
	addr, err := redeemPayment.WitnessScriptHash()
	if err != nil {
		return "", err
	}
	return addr, nil
}

