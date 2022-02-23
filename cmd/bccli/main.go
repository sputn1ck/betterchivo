package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/btcsuite/btcd/btcec"
	"github.com/sputn1ck/liquid-go-lightwallet/lightning"
	"google.golang.org/grpc"
	"log"
	"os"
	"strconv"

	"github.com/sputn1ck/liquid-go-lightwallet/swaprpc"
)

var usdt = "asdf"

func main() {
	if err := run(); err != nil {
		log.Printf("Error: %v", err)
	}
}

func run() error {
	if len(os.Args) < 2 {
		return errors.New("expected amount ")
	}
	amount, err := strconv.Atoi(os.Args[1])
	if err != nil {
		return err
	}

	conn, err := getClientConn("localhost:42069")
	if err != nil {
		return err
	}
	defer conn.Close()

	psClient := swaprpc.NewSwapServiceClient(conn)

	dummyWallet := &DummyWallet{}

	bcc := &BetterChivoClient{
		rpc:    psClient,
		wallet: dummyWallet,
	}

	err = bcc.receiveUsdt(uint64(amount))
	if err != nil {
		return err
	}
	return nil

}


func getClientConn(address string) (*grpc.ClientConn, error) {

	maxMsgRecvSize := grpc.MaxCallRecvMsgSize(1 * 1024 * 1024 * 200)
	opts := []grpc.DialOption{
		grpc.WithDefaultCallOptions(maxMsgRecvSize),
		grpc.WithInsecure(),
	}

	conn, err := grpc.Dial(address, opts...)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to RPC server: %v",
			err)
	}

	return conn, nil
}


type BetterChivoClient struct {
	rpc swaprpc.SwapServiceClient
	wallet Wallet
}


func (client *BetterChivoClient) receiveUsdt(amount uint64) error{
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

    // create claim key
	privkey, err := btcec.NewPrivateKey(btcec.S256())
	if err != nil {
		return err
	}
	pubkey := privkey.PubKey().SerializeCompressed()

	// create preimage
	preimage, err := lightning.GetPreimage()
	if err != nil {
		return err
	}

	phash := preimage.Hash()
	// get address
	address := client.wallet.GetNewAddress()
	// get asset

	// send request

	stream, err := client.rpc.ReceivePayment(ctx)
	if err != nil {
		return err
	}

	// send request
	msg := &swaprpc.ReceivePaymentRequest{
		Message: &swaprpc.ReceivePaymentRequest_StartReceive{&swaprpc.StartReceiveMessage{
			PaymentHash: phash[:],
			TakerPubkey:  pubkey,
			Amount:      amount,
			Asset:       usdt,
			Address: address,
		}},
	}
	err = stream.Send(msg)
	if err != nil {
		return err
	}

	// wait for waitforpayment message
	res, err := stream.Recv()
	if err != nil {
		return err
	}

	waitForPayment := res.GetWaitForPayment()
	if waitForPayment == nil {
		return errors.New("expected wait for payment message")
	}

	// now we show the invoice
	log.Printf("Invoice: %s", waitForPayment.Invoice)

	// now we wait for the txopened message
	res, err = stream.Recv()
	if err != nil {
		return err
	}

	txopened := res.GetTxOpened()
	if txopened == nil {
		return errors.New("expected wait for payment message")
	}

	// we now claim the tx
	swapParams := NewPreimageClaimParams(txopened.MakerPubkey, pubkey, txopened.Csv,preimage[:], phash[:], privkey)
	txId, err := client.wallet.ClaimSwap(swapParams)
	if err != nil {
		return err
	}

	log.Printf("claimed swap: %s", txId)

	msg = &swaprpc.ReceivePaymentRequest{
		Message: &swaprpc.ReceivePaymentRequest_PreimageMessage{&swaprpc.PreimageMessage{
			Preimage: preimage[:],
		}},
	}

	err = stream.Send(msg)
	if err != nil {
		return err
	}

	log.Printf("swap completed, received %v of %s", amount, usdt)
	return nil
}

type Wallet interface {
	GetNewAddress() string
	ClaimSwap(claimParams PreimageClaimParams) (string, error)
}

type PreimageClaimParams struct {
	makerPubkey []byte
	takerPubkey []byte
	csv uint32
	preimage []byte
	phash []byte
	signingKey *btcec.PrivateKey
}

func NewPreimageClaimParams(makerPubkey []byte, takerPubkey []byte, csv uint32, preimage []byte, phash []byte, signingKey *btcec.PrivateKey) PreimageClaimParams {
	return PreimageClaimParams{makerPubkey: makerPubkey, takerPubkey: takerPubkey, csv: csv, preimage: preimage, phash: phash, signingKey: signingKey}
}


type DummyWallet struct {}

func (d DummyWallet) GetNewAddress() string {
	return "address"
}

func (d DummyWallet) ClaimSwap(claimParams PreimageClaimParams) (string, error) {
	return "txid", nil
}
