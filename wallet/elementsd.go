package wallet

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ybbus/jsonrpc"
	"log"
	"net/url"
	"strings"
)

type ElementsdClient struct {
	baseUrl string
	username string
	password string
	Rpc jsonrpc.RPCClient
}

type GetAddressInfo struct {
	Unconfidential string `json:"unconfidential"`
}
func (e *ElementsdClient) GetNewAddress(addrType int) (string, error) {
	res, err := e.Rpc.Call("getnewaddress")
	if err != nil {
		return "", err
	}
	addr, err := res.GetString()
	if err != nil {
		return "", err
	}
	var addrInfo *GetAddressInfo
	err = e.Rpc.CallFor(&addrInfo, "getaddressinfo", addr)
	if err != nil {
		return "", err
	}

	return addrInfo.Unconfidential, nil
}

type FundRawtransactionRes struct {
	TxHex string `json:"hex"`
}

func (e *ElementsdClient) FundRawTransaction(unfundedTxHex string) (string, error) {
	var fundRes *FundRawtransactionRes
	err := e.Rpc.CallFor(&fundRes, "fundrawtransaction", unfundedTxHex)
	if err != nil {
		return "", err
	}

	return fundRes.TxHex, nil
}

func (e *ElementsdClient) SignRawTransactionWithWallet(unsignedTxhex string) (string, error) {
	var signRes *FundRawtransactionRes
	err := e.Rpc.CallFor(&signRes, "signrawtransactionwithwallet", unsignedTxhex)
	if err != nil {
		return "", err
	}

	return signRes.TxHex, nil
}

func (e *ElementsdClient) BlindRawTransaction(unsignedTxhex string) (string, error) {
	var signRes *string
	err := e.Rpc.CallFor(&signRes, "blindrawtransaction", unsignedTxhex)
	if err != nil {
		return "", err
	}

	return *signRes, nil
}

func (e *ElementsdClient) SendToAddress(address string, asset string, amount string) (string, error) {
	params := map[string]interface{}{
		"mempool_sequence": true,
	}
	res, err := e.Rpc.Call("sendtoaddress", address, amount, params)
	if err != nil {
		return "", err
	}
	if res.Error != nil {
		return "", res.Error
	}
	return res.GetString()
}

type BalanceRes struct {
	Assets map[string]float64 `json:"bitcoin"`
}
func (e *ElementsdClient) GetBalance(asset string) (float64, error) {
	//var balanceRes *BalanceRes
	res, err := e.Rpc.Call("getbalance")
	if err != nil {
		return 0, err
	}
	if res.Error != nil {
		return 0, res.Error
	}


	balanceMap := res.Result.(map[string]interface{})
	var val interface{}
	var ok bool

	if val, ok = balanceMap[asset]; !ok {
		return 0, nil
	}
	var number json.Number
	if number, ok = val.(json.Number); !ok {
		return 0,nil
	}



	return number.Float64()
}

type WalletRes struct {
 Name string `json:"wallet"`
}
func (e *ElementsdClient) LoadWallet(filename string) (string, error) {
	var walletRes *WalletRes
	err := e.Rpc.CallFor(&walletRes, "loadwallet", filename)
	if err != nil {
		return "", err
	}
	return walletRes.Name, nil
}

func (e *ElementsdClient) CreateWallet(walletname string) (string, error) {
	var walletRes *WalletRes
	err := e.Rpc.CallFor(&walletRes, "createwallet", walletname)
	if err != nil {
		return "", err
	}
	return walletRes.Name, nil
}

func (e *ElementsdClient) ListWallets() ([]string, error) {
	wallets := []string{}
	res, err := e.Rpc.Call("listwallets")
	if err != nil {
		return nil, err
	}
	err = res.GetObject(&wallets)
	if err != nil {
		return nil, err
	}
	return wallets, nil
}


func (e *ElementsdClient) SendRawTransaction(txHex string) (string, error) {
	log.Printf("calling sendraw")
	res, err := e.Rpc.Call("sendrawtransaction", txHex)
	if err != nil {
		return "", err
	}
	if res.Error != nil {
		return "", res.Error
	}
	return res.GetString()
}

func NewElementsdClient(baseUrl, user, password string) (*ElementsdClient, error) {
	serviceRawURL := fmt.Sprintf("%s://%s", "http", baseUrl)
	serviceURL, err := url.Parse(serviceRawURL)
	if err != nil {
		return nil, fmt.Errorf("url.Parse() %w", err)
	}

	authPair := fmt.Sprintf("%s:%s", user, password)
	authPairb64 := base64.RawURLEncoding.EncodeToString([]byte(authPair))
	authHeader := []byte("Basic ")
	authHeader = append(authHeader, []byte(authPairb64)...)

	rpcClient := jsonrpc.NewClientWithOpts(serviceURL.String(), &jsonrpc.RPCClientOpts{
		CustomHeaders: map[string]string{
			"Authorization": string(authHeader),
		},
	})
	return &ElementsdClient{Rpc: rpcClient, username: user, baseUrl: baseUrl, password: password}, nil
}

func (e *ElementsdClient) SetRpcWallet(walletname string) error {
	serviceRawURL := fmt.Sprintf("%s://%s/wallet/%s", "http", e.baseUrl, walletname)
	serviceURL, err := url.Parse(serviceRawURL)
	if err != nil {
		return fmt.Errorf("url.Parse() %w", err)
	}

	authPair := fmt.Sprintf("%s:%s", e.username, e.password)
	authPairb64 := base64.RawURLEncoding.EncodeToString([]byte(authPair))
	authHeader := []byte("Basic ")
	authHeader = append(authHeader, []byte(authPairb64)...)

	e.Rpc = jsonrpc.NewClientWithOpts(serviceURL.String(), &jsonrpc.RPCClientOpts{
		CustomHeaders: map[string]string{
			"Authorization": string(authHeader),
		},
	})
	return nil
}


var (
	AlreadyExistsError = errors.New("wallet already exists")
	AlreadyLoadedError = errors.New("wallet is already loaded")
)


// ElementsRpcWallet uses the elementsd rpc wallet
type ElementsRpcWallet struct {
	walletName string
	rpcClient  *ElementsdClient
}

func NewRpcWallet(rpcClient *ElementsdClient, walletName string) (*ElementsRpcWallet, error) {
	rpcWallet := &ElementsRpcWallet{
		walletName: walletName,
		rpcClient:  rpcClient,
	}
	err := rpcWallet.setupWallet()
	if err != nil {
		return nil, err
	}
	return rpcWallet, nil
}


// setupWallet checks if the swap wallet is already loaded in elementsd, if not it loads/creates it
func (r *ElementsRpcWallet) setupWallet() error {
	loadedWallets, err := r.rpcClient.ListWallets()
	if err != nil {
		return err
	}
	var walletLoaded bool
	for _, v := range loadedWallets {
		if v == r.walletName {
			walletLoaded = true
			break
		}
	}
	if !walletLoaded {
		_, err = r.rpcClient.LoadWallet(r.walletName)
		if err != nil && (strings.Contains(err.Error(), "Wallet file verification failed") || strings.Contains(err.Error(), "not found")) {
			_, err = r.rpcClient.CreateWallet(r.walletName)
			if err != nil {
				return err
			}
		} else if err != nil {
			return err
		}

	}
	err = r.rpcClient.SetRpcWallet(r.walletName)
	if err != nil {
		return err
	}
	return nil
}

// GetBalance returns the balance in sats
func (r *ElementsRpcWallet) GetBalance(asset string) (float64, error) {
	balance, err := r.rpcClient.GetBalance(asset)
	if err != nil {
		return 0, err
	}
	return balance, nil
}

// GetAddress returns a new blech32 address
func (r *ElementsRpcWallet) GetAddress() (string, error) {
	address, err := r.rpcClient.GetNewAddress(0)
	if err != nil {
		return "", err
	}
	return address, nil
}

// SendToAddress sends an amount to an address
func (r *ElementsRpcWallet) SendToAddress(address string, amount uint64, asset string) (string, error) {
	txId, err := r.rpcClient.SendToAddress(address, satsToAmountString(amount), asset)
	if err != nil {
		return "", err
	}
	return txId, nil
}

func (r *ElementsRpcWallet) FundAndSignRawTransaction(unfundedRawTx string) (string, error) {
	fundedTx, err := r.rpcClient.FundRawTransaction(unfundedRawTx)
	if err != nil {
		return "", err
	}

	blindedRawTx, err := r.rpcClient.BlindRawTransaction(fundedTx)
	if err != nil {
		return "", err
	}
	signedTx, err := r.rpcClient.SignRawTransactionWithWallet(blindedRawTx)
	if err != nil {
		return "", err
	}

	return signedTx, nil
}

func (r *ElementsRpcWallet) SendRawTransaction(txHex string) (string, error) {
	return r.rpcClient.SendRawTransaction(txHex)
}

// satsToAmountString returns the amount in btc from sats
func satsToAmountString(sats uint64) string {
	bitcoinAmt := float64(sats) / 100000000
	return fmt.Sprintf("%f", bitcoinAmt)
}

