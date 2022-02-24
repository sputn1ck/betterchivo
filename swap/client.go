package swap

import (
	"context"
	"errors"
	"github.com/btcsuite/btcd/btcec"
	"github.com/sputn1ck/liquid-go-lightwallet/chain"
	"github.com/sputn1ck/liquid-go-lightwallet/lightning"
	"github.com/sputn1ck/liquid-go-lightwallet/swaprpc"
	"log"
)


type Wallet interface {
	GetAddress() (string, error)
	SendRawTransaction(txHex string) (string, error)
}

type Blockchain interface {
	CreatePreimageSpendingTransaction(params chain.ClaimParams) (string, error)
}

type BetterChivoClient struct {
	rpc swaprpc.SwapServiceClient
	wallet Wallet
	chain Blockchain
}

func NewBetterChivoClient(rpc swaprpc.SwapServiceClient, wallet Wallet, chain Blockchain) *BetterChivoClient {
	return &BetterChivoClient{rpc: rpc, wallet: wallet, chain: chain}
}


func (client *BetterChivoClient) ReceiveUsdt(amount uint64, asset []byte) error{
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
			Asset:       asset,
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

	// get address
	address,err := client.wallet.GetAddress()
	if err != nil {
		return err
	}
	log.Printf("maker pubkey: %x, takerpubkey: %x", txopened.MakerPubkey, pubkey)

	swapParams := chain.NewClaimParams(txopened.TxHex,address, amount, txopened.Csv,txopened.MakerPubkey, pubkey,preimage[:], phash[:],asset, privkey)
	claimTxHex, err := client.chain.CreatePreimageSpendingTransaction(swapParams)
	if err != nil {
		return err
	}
	txId, err := client.wallet.SendRawTransaction(claimTxHex)
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

	log.Printf("swap completed, received %v of %x", amount, asset)
	return nil
}


