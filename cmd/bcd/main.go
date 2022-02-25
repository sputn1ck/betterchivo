package main

import (
	"context"
	"github.com/sputn1ck/liquid-go-lightwallet/chain"
	"github.com/sputn1ck/liquid-go-lightwallet/lightning"
	"github.com/sputn1ck/liquid-go-lightwallet/swap"
	"github.com/sputn1ck/liquid-go-lightwallet/swaprpc"
	"github.com/sputn1ck/liquid-go-lightwallet/wallet"
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
	lndconnect = "lndconnect://127.0.0.1:10001?cert=MIICJzCCAc2gAwIBAgIRAM8SaqbghgiYTb5ZLIMNKiMwCgYIKoZIzj0EAwIwMTEfMB0GA1UEChMWbG5kIGF1dG9nZW5lcmF0ZWQgY2VydDEOMAwGA1UEAxMFYWxpY2UwHhcNMjIwMjI0MTQwNzAzWhcNMjMwNDIxMTQwNzAzWjAxMR8wHQYDVQQKExZsbmQgYXV0b2dlbmVyYXRlZCBjZXJ0MQ4wDAYDVQQDEwVhbGljZTBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABBSvPfCLcRu9xss932o44ezvdG9ObGAamzt4QHaeuYN2hq8tZ34BXTi1PC73lHyWdNNG4r2Vk-KXG_cFHhwMKmOjgcUwgcIwDgYDVR0PAQH_BAQDAgKkMBMGA1UdJQQMMAoGCCsGAQUFBwMBMA8GA1UdEwEB_wQFMAMBAf8wHQYDVR0OBBYEFK46_ewGBrWfwQz1rSUYqHCBR5FYMGsGA1UdEQRkMGKCBWFsaWNlgglsb2NhbGhvc3SCBWFsaWNlgg5wb2xhci1uMS1hbGljZYIEdW5peIIKdW5peHBhY2tldIIHYnVmY29ubocEfwAAAYcQAAAAAAAAAAAAAAAAAAAAAYcErBUABDAKBggqhkjOPQQDAgNIADBFAiEA5_TuZG9JXVWGwVjvWLhjzI-lwnkemC25JhumAMVVCZUCICEFm2JhhCumljkx5UGFM-Lhjr-ChmfyJ_jcrdQUzjCk&macaroon=AgEDbG5kAvgBAwoQrKvUd3mu8R0buI_mOrP-1RIBMBoWCgdhZGRyZXNzEgRyZWFkEgV3cml0ZRoTCgRpbmZvEgRyZWFkEgV3cml0ZRoXCghpbnZvaWNlcxIEcmVhZBIFd3JpdGUaIQoIbWFjYXJvb24SCGdlbmVyYXRlEgRyZWFkEgV3cml0ZRoWCgdtZXNzYWdlEgRyZWFkEgV3cml0ZRoXCghvZmZjaGFpbhIEcmVhZBIFd3JpdGUaFgoHb25jaGFpbhIEcmVhZBIFd3JpdGUaFAoFcGVlcnMSBHJlYWQSBXdyaXRlGhgKBnNpZ25lchIIZ2VuZXJhdGUSBHJlYWQAAAYgexb5fqA5XsR6_DcvO6my1xaRs8xXzhTeqcA85A-XPms"
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// set up lightning
	lnd, err := lightning.NewLnd(ctx, lndconnect)
	if err != nil {
		return err
	}
	rpcClient, err := wallet.NewElementsdClient("localhost:18884", "admin1","123")
	if err != nil {
		return err
	}

	liquidWallet,err := wallet.NewRpcWallet(rpcClient, "betterchivo-server")
	if err != nil {
		return err
	}

	unblindedAddr, err := liquidWallet.GetAddress()
	if err != nil {
		return err
	}

	log.Printf("Server unblinded address: %s", unblindedAddr)

	liquidChain := chain.NewLiquidOnchain(liquidNetwork)
	dummyCC := &DummyCurrencyConverter{}

	swapServer := swap.NewBetterChivoServer(liquidWallet, lnd,liquidChain,dummyCC)
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

//func runOld() error {
//	seed := bip39.NewSeed(accounts[0], "")
//
//	cfgCfgParams := blockchain.GetChainCfgParams(liquidNetwork, &chaincfg.MainNetParams)
//
//	esplora := chain.NewEsploraApi("http://localhost:3001")
//
//	wallet := wallet.NewLiquidWallet(esplora, cfgCfgParams, liquidNetwork)
//	err := wallet.Initialize(seed)
//	if err != nil {
//		return err
//	}
//	lastAddr, err := wallet.GetAddress()
//	if err != nil {
//		return err
//	}
//	fmt.Printf("next address %s \n", lastAddr)
//
//	//inputs, totalValue, err := wallet.GetInputs("", 50000)
//	//if err != nil {
//	//	return err
//	//}
//	//log.Printf("inputs: totalValue %v addr[0]: %v \n", totalValue, inputs[0].Address)
//	utxos, err := wallet.GetUtxos()
//	for _, v:= range utxos {
//		log.Printf("%s", v)
//	}
//	res, err := wallet.SendToAddress(lastAddr,"",50000)
//	if err != nil {
//		return err
//	}
//	log.Printf("%s", res)
//
//	return nil
//}





type DummyCurrencyConverter struct {}

func (d *DummyCurrencyConverter) GetSatAmt(asset string, amount uint64) (uint64, error) {
	return amount, nil
}



