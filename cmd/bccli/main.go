package main

import (
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/sputn1ck/liquid-go-lightwallet/chain"
	"github.com/sputn1ck/liquid-go-lightwallet/swap"
	"github.com/sputn1ck/liquid-go-lightwallet/wallet"
	"github.com/vulpemventures/go-elements/network"
	"google.golang.org/grpc"
	"log"
	"os"
	"strconv"

	"github.com/sputn1ck/liquid-go-lightwallet/swaprpc"
)

var liquidNetwork = &network.Regtest

var usdt = "2dcf5a8834645654911964ec3602426fd3b9b4017554d3f9c19403e7fc1411d3"

var seed = "blossom must cherry inform whale steak wish raw arm among run dog middle animal horse history sustain extra trend walnut orchard grass bid caution"

var helpMsg = "you need to provice a command (newaddress, sendtoaddress, receive 'amt in usdt'"

func main() {
	if len(os.Args) < 2 {
		log.Printf(helpMsg)
		return
	}

	switch os.Args[1] {
	case "receive":
		if err := receive(); err != nil {
			log.Printf("Error: %v", err)
		}
	case "newaddress":
		if err := getAddress(); err != nil {
			log.Printf("Error: %v", err)
		}
	case "balance":
		if err := getBalance(); err != nil {
			log.Printf("Error: %v", err)
		}
	default:
		log.Printf(helpMsg)
	}


}

func getBalance() error {
	rpcClient, err := wallet.NewElementsdClient("localhost:18884", "admin1", "123")
	if err != nil {
		return err
	}

	liquidWallet, err := wallet.NewRpcWallet(rpcClient, "betterchivo-client")
	if err != nil {
		return err
	}
	balance, err := liquidWallet.GetBalance(usdt)
	if err != nil {
		return err
	}
	log.Printf("USDT Balance: %v", balance)
	return nil
}
func getAddress() error {
	rpcClient, err := wallet.NewElementsdClient("localhost:18884", "admin1","123")
	if err != nil {
		return err
	}

	liquidWallet,err := wallet.NewRpcWallet(rpcClient, "betterchivo-client")
	if err != nil {
		return err
	}
	address, err := liquidWallet.GetAddress()
	if err != nil {
		return err
	}
	log.Printf("%s", address)
	return nil
}



func receive() error {
	if len(os.Args) < 3 {
		return errors.New("expected amount ")
	}
	amount, err := strconv.Atoi(os.Args[2])
	if err != nil {
		return err
	}

	conn, err := getClientConn("localhost:42069")
	if err != nil {
		return err
	}
	defer conn.Close()

	psClient := swaprpc.NewSwapServiceClient(conn)

	rpcClient, err := wallet.NewElementsdClient("localhost:18884", "admin1","123")
	if err != nil {
		return err
	}

	liquidWallet,err := wallet.NewRpcWallet(rpcClient, "betterchivo-client")
	if err != nil {
		return err
	}

	blockchain := chain.NewLiquidOnchain(liquidNetwork)

	bcc := swap.NewBetterChivoClient(psClient, liquidWallet, blockchain)

	usdtBytes, err := hex.DecodeString(usdt)
	if err != nil {
		return err
	}

	err = bcc.ReceiveUsdt(uint64(amount), blockchain.TranslateAsset(usdtBytes))
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




