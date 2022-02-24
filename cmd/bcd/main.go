package main

import (
	"github.com/sputn1ck/liquid-go-lightwallet/chain"
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
	dummyNode := &DummyLightningNode{}
	dummyCC := &DummyCurrencyConverter{}

	swapServer := swap.NewBetterChivoServer(liquidWallet, dummyNode,liquidChain,dummyCC)
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



