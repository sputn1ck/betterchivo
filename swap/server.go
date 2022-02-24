package swap

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"github.com/btcsuite/btcd/btcec"
	"github.com/sputn1ck/liquid-go-lightwallet/chain"
	"github.com/sputn1ck/liquid-go-lightwallet/swaprpc"
	"log"
)

const (
	SWAP_CSV = 30
)

type LightningWallet interface {
	CreateHodlInvoice(amount uint64) (string, error)
	WaitforPaymentAccepted(invoice string) error
	SettleInvoice(invoice string, preimage []byte) error
}

type CurrencyConverter interface {
	GetSatAmt(asset string, amount uint64) (uint64, error)
}

type SwapWallet interface {
	//AddWaitForPreimageReveal(txId string) (chan []byte)
	SendToAddress(address string,  amount uint64, asset string) (string, error)
	SendRawTransaction(txHex string) (string, error)
	FundAndSignRawTransaction(unfundedRawTx string) (string, error)
	GetBalance(asset string) (uint64, error)
}

type OpeningTxCreator interface {
	CreateUnfundedOpeningTransaction(params chain.SwapOpeningParams) (string, error)
	GetAsset() []byte
}


type BetterChivoServer struct {
	wallet SwapWallet
	node   LightningWallet
	blockchain OpeningTxCreator
	cc CurrencyConverter

	swaprpc.UnimplementedSwapServiceServer
}

func NewBetterChivoServer(wallet SwapWallet, node LightningWallet, blockchain OpeningTxCreator, cc CurrencyConverter) *BetterChivoServer {
	return &BetterChivoServer{wallet: wallet, node: node, blockchain: blockchain, cc: cc}
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
	log.Printf("[%s] New receive request: Amount: %v Asset: %s" , swapId, startReceiveRequest.Amount, startReceiveRequest.Asset)

	// todo check balance
	_, err = b.wallet.GetBalance(hex.EncodeToString(startReceiveRequest.Asset))
	if err != nil {
		return err
	}

	// create privkey for swap
	privkey, err := btcec.NewPrivateKey(btcec.S256())
	if err != nil {
		return err
	}
	pubkey := privkey.PubKey().SerializeCompressed()

	// get satamt
	satAmt, err := b.cc.GetSatAmt(hex.EncodeToString(startReceiveRequest.Asset), startReceiveRequest.Amount)
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
	log.Printf("maker pubkey: %x, takerpubkey: %x", pubkey, startReceiveRequest.TakerPubkey)
	openingParams := chain.NewSwapOpeningParams(pubkey, startReceiveRequest.TakerPubkey, SWAP_CSV, startReceiveRequest.PaymentHash,[]chain.AssetAmountTuple{{b.blockchain.GetAsset(),500},{startReceiveRequest.Asset, startReceiveRequest.Amount}})
	unfinishedTxHex, err := b.blockchain.CreateUnfundedOpeningTransaction(openingParams)
	if err != nil {
		return err
	}
	finishedTxHex, err := b.wallet.FundAndSignRawTransaction(unfinishedTxHex)
	if err != nil {
		return err
	}

	txId, err := b.wallet.SendRawTransaction(finishedTxHex)
	if err != nil {
		return err
	}

	msg = &swaprpc.ReceivePaymentResponse {
		Message: &swaprpc.ReceivePaymentResponse_TxOpened{
			TxOpened: &swaprpc.TxOpenedMessage{
				TxId: txId,
				Csv: SWAP_CSV,
				MakerPubkey: pubkey,
				TxHex: finishedTxHex,
			},
		},
	}

	err = server.Send(msg)
	if err != nil {
		return err
	}

	log.Printf("[%s] Sent tx opened message: TxId: %s",swapId, txId)
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


// newSwapId returns a random 32 byte hex string
func newSwapId() string {
	idBytes := make([]byte, 16)
	_, _ = rand.Read(idBytes[:])
	return hex.EncodeToString(idBytes)
}

