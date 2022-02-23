package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/sputn1ck/liquid-go-lightwallet/chain"
	"github.com/sputn1ck/liquid-go-lightwallet/swap"
	"github.com/sputn1ck/liquid-go-lightwallet/swaprpc"
	"github.com/sputn1ck/liquid-go-lightwallet/wallet"
	"github.com/tyler-smith/go-bip39"
	"github.com/vulpemventures/go-elements/network"
	"google.golang.org/grpc"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
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

	shutdown := make(chan struct{})
	sigChan := make(chan os.Signal)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		defer close(shutdown)
		defer close(sigChan)

		select {
		case sig := <-sigChan:
			log.Printf("received signal: %v, release shutdown", sig)
			shutdown <- struct{}{}
		}
	}()

	dummyWallet := &DummyWallet{}
	dummyNode := &DummyLightningNode{}
	dummyCC := &DummyCurrencyConverter{}

	swapServer := &BetterChivoServer{
		wallet:                         dummyWallet,
		node:                           dummyNode,
		cc:                             dummyCC,
	}
	host := "localhost:42069"
	lis, err := net.Listen("tcp", host)
	if err != nil {
		return err
	}
	defer lis.Close()

	grpcSrv := grpc.NewServer()

	swaprpc.RegisterSwapServiceServer(grpcSrv, swapServer)

	go func() {
		err := grpcSrv.Serve(lis)
		if err != nil {
			log.Fatal(err)
		}
	}()
	defer grpcSrv.GracefulStop()
	log.Printf("betterchivod listening on %v", host)
	<-shutdown
	return nil
}

func runOld() error {
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

type BetterChivoServer struct {
	wallet SwapWallet
	node   LightningWallet
	cc CurrencyConverter

	swaprpc.UnimplementedSwapServiceServer
}

func (b *BetterChivoServer) GetRates(ctx context.Context, request *swaprpc.GetRatesRequest) (*swaprpc.GetRatesResponse, error) {
	panic("implement me")
}

func (b *BetterChivoServer) SendPayment(server swaprpc.SwapService_SendPaymentServer) error {
	panic("implement me")
}

func (b *BetterChivoServer) ReceivePayment(server swaprpc.SwapService_ReceivePaymentServer) error {
	swapId := newSwapId()
	recv, err := server.Recv()
	if err != nil {
		return err
	}

	startReceiveRequest := recv.GetStartReceive()
	if startReceiveRequest == nil {
		return errors.New("expected StartReceive message")
	}
	log.Printf("[%s] New receive request: Amount: %v Asset: %s Address: %s", swapId, startReceiveRequest.Amount, startReceiveRequest.Asset, startReceiveRequest.Address)

	// create privkey for swap
	privkey, err := btcec.NewPrivateKey(btcec.S256())
	if err != nil {
		return err
	}
	pubkey := privkey.PubKey().SerializeCompressed()

	// get satamt
	satAmt, err := b.cc.GetSatAmt(startReceiveRequest.Asset, startReceiveRequest.Amount)
	if err != nil {
		return err
	}

	// Create Invoice
	invoice, err := b.node.CreateHodlInvoice(satAmt)
	if err != nil {
		return err
	}

	acceptedChan := make(chan error)
	go func(){
		err = b.node.WaitforPaymentAccepted(invoice)
		acceptedChan<-err
	}()

	msg := &swaprpc.ReceivePaymentResponse {
		Message: &swaprpc.ReceivePaymentResponse_WaitForPayment{
			WaitForPayment: &swaprpc.WaitForPaymentMessage{
				Invoice: invoice,
				SwapId: swapId,
			},
		},
	}

	err = server.Send(msg)
	if err != nil {
		return err
	}
	log.Printf("[%s] Sent wait for payment message: Invoice: %s",swapId, invoice)

	// wait for payment
	waitLoop:
	for {
		select {
			case <-server.Context().Done():
				return errors.New("context done")
			case paymentErr := <-acceptedChan:
				if paymentErr == nil {
					break waitLoop
				}
				return paymentErr
		}
	}

	log.Printf("[%s] Payment accepted", swapId)
	// if payment has been accepted, we open the swap
	openingParams := NewSwapOpeningParams(pubkey, startReceiveRequest.TakerPubkey, swap.SWAP_CSV, startReceiveRequest.PaymentHash)
	txInfo, err := b.wallet.CreateSwapTransaction(openingParams)
	if err != nil {
		return err
	}

	msg = &swaprpc.ReceivePaymentResponse {
		Message: &swaprpc.ReceivePaymentResponse_TxOpened{
			TxOpened: &swaprpc.TxOpenedMessage{
				TxId: txInfo.txId,
				TxHex: txInfo.txHex,
				ScriptVout: txInfo.scriptVout,
				Csv: swap.SWAP_CSV,
				MakerPubkey: pubkey,
			},
		},
	}

	err = server.Send(msg)
	if err != nil {
		return err
	}

	log.Printf("[%s] Sent tx opened message: TxId: %s",swapId, txInfo.txId)
	// TODO: find out preimage from onchain
	// now we wait for the preimage

	recv, err = server.Recv()
	if err != nil {
		return err
	}

	preimageMessage := recv.GetPreimageMessage()
	if preimageMessage == nil {
		return errors.New("expected preimage message")
	}

	log.Printf("[%s] Received invoice: %x",swapId, preimageMessage.Preimage)
	err = b.node.SettleInvoice(invoice, preimageMessage.Preimage)
	if err != nil {
		return err
	}


	log.Printf("[%s] Swap done", swapId)
	return nil
}

type SwapWallet interface {
	CreateSwapTransaction(openingParams SwapOpeningParams) (TxInfo, error)
	ClaimCsvTransaction(openingParams SwapOpeningParams, signingKey *btcec.PrivateKey) (string, error)
	AddWaitForPreimageReveal(txId string) (chan []byte)
}

type SwapOpeningParams struct {
	makerPubkey []byte
	takerPubkey []byte
	csv uint32
	phash []byte
}

func NewSwapOpeningParams(makerPubkey []byte, takerPubkey []byte, csv uint32, phash []byte) SwapOpeningParams {
	return SwapOpeningParams{makerPubkey: makerPubkey, takerPubkey: takerPubkey, csv: csv, phash: phash}
}

type CsvClaimParams struct {
	makerPubkey []byte
	takerPubkey []byte
	csv uint32
	phash []byte
	signingKey *btcec.PrivateKey
}


type TxInfo struct {
	txHex string
	txId string
	scriptVout uint32
}

type LightningWallet interface {
	CreateHodlInvoice(amount uint64) (string, error)
	WaitforPaymentAccepted(invoice string) error
	SettleInvoice(invoice string, preimage []byte) error
}

type CurrencyConverter interface {
	GetSatAmt(asset string, amount uint64) (uint64, error)
}

type DummyWallet struct {}

func (d *DummyWallet)CreateSwapTransaction(openingParams SwapOpeningParams) ( TxInfo, error) {
	return TxInfo{
		txHex:      "txhex",
		txId:       "txid",
		scriptVout: 0,
	},nil
}

func (d *DummyWallet)ClaimCsvTransaction(openingParams SwapOpeningParams, signingKey *btcec.PrivateKey) ( string, error) {
	panic("implement me")
}

func (d *DummyWallet)AddWaitForPreimageReveal(txId string) chan []byte {
	panic("implement me")
}

type DummyLightningNode struct {
}

func (d *DummyLightningNode) CreateHodlInvoice(amount uint64) (string, error) {
	return "invoice", nil
}

func (d *DummyLightningNode) WaitforPaymentAccepted(invoice string) (error) {
	return nil
}

func (d *DummyLightningNode) SettleInvoice(invoice string, preimage []byte) error {
	return nil
}

type DummyCurrencyConverter struct {}

func (d *DummyCurrencyConverter) GetSatAmt(asset string, amount uint64) (uint64, error) {
	return amount, nil
}

// newSwapId returns a random 32 byte hex string
func newSwapId() string {
	idBytes := make([]byte, 16)
	_, _ = rand.Read(idBytes[:])
	return hex.EncodeToString(idBytes)
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



