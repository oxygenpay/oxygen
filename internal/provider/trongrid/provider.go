package trongrid

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/oxygenpay/oxygen/internal/util"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/tidwall/gjson"
	"golang.org/x/sync/errgroup"
)

type Config struct {
	MainnetBaseURL string `yaml:"mainnet_url" env:"TRONGRID_MAINNET_URL" env-default:"https://api.trongrid.io" env-description:"Trongrid API base path"`
	TestnetBaseURL string `yaml:"testnet_url" env:"TRONGRID_TESTNET_URL" env-default:"https://api.shasta.trongrid.io" env-description:"Trongrid testnet (shasta) API base path"`
	APIKey         string `yaml:"api_key" env:"TRONGRID_API_KEY" env-description:"Trongrid API Key"`
}

type Provider struct {
	config Config
	logger *zerolog.Logger
	client http.Client
}

// TransactionRequest transaction request.
// Address format: 410fe47f49fd91f0edb7fa2b94a3c45d9c2231709c
// See https://developers.tron.network/reference/createtransaction
type TransactionRequest struct {
	OwnerAddress string `json:"owner_address"`
	ToAddress    string `json:"to_address"`
	Amount       uint64 `json:"amount"`
	ExtraData    string `json:"extra_data"`
	Visible      bool   `json:"visible"`
}

type Transaction struct {
	TxID       string          `json:"txID,omitempty"`
	RawDataHex string          `json:"raw_data_hex,omitempty"`
	RawData    json.RawMessage `json:"raw_data,omitempty"`
	Visible    bool            `json:"visible"`

	Signature []string `json:"signature,omitempty"`

	Error string `json:"Error,omitempty"`
}

type ContractCallRequest struct {
	OwnerAddress     string `json:"owner_address"`
	ContractAddress  string `json:"contract_address"`
	FunctionSelector string `json:"function_selector"`
	Parameter        string `json:"parameter"`
	FeeLimit         uint64 `json:"fee_limit"`
	CallValue        uint64 `json:"call_value"`
	Visible          bool   `json:"visible"`
}

const (
	//nolint:gosec
	headerAPIKey       = "TRON-PRO-API-KEY"
	confirmationBlocks = 10
)

var (
	ErrResponse = errors.New("error response")
	ErrNotFound = errors.New("transaction not found")
)

func New(cfg Config, logger *zerolog.Logger) *Provider {
	log := logger.With().Str("channel", "trongrid_provider").Logger()

	cfg.MainnetBaseURL = strings.TrimRight(cfg.MainnetBaseURL, "/")
	cfg.TestnetBaseURL = strings.TrimRight(cfg.TestnetBaseURL, "/")

	return &Provider{
		config: cfg,
		client: http.Client{
			Timeout: time.Second * 5,
		},
		logger: &log,
	}
}

// CreateTransaction fcking TRON makes offline tx creation so hard
// so they basically implemented endpoint for unsigned tx creation
func (p *Provider) CreateTransaction(
	ctx context.Context, payload TransactionRequest, isTest bool,
) (Transaction, error) {
	req, err := p.newRequest(ctx, http.MethodPost, "/wallet/createtransaction", payload, isTest)
	if err != nil {
		return Transaction{}, errors.Wrap(err, "unable to create request")
	}

	res, err := p.client.Do(req)
	if err != nil {
		return Transaction{}, errors.Wrap(err, "response error")
	}

	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return Transaction{}, errors.Wrap(err, "unable to read response")
	}

	p.logger.Info().
		Interface("request", payload).
		Str("url", req.URL.String()).
		RawJSON("response", body).
		Int("response_code", res.StatusCode).
		Msg("CreateTransaction response")

	if res.StatusCode != http.StatusOK {
		return Transaction{}, errors.Wrapf(ErrResponse, "got %d response code", res.StatusCode)
	}

	var txRes Transaction
	if err := json.Unmarshal(body, &txRes); err != nil {
		return Transaction{}, errors.Wrap(err, "unmarshal error")
	}

	return txRes, nil
}

// CallResponse represents response when calling /wallet/triggersmartcontract.
type CallResponse struct {
	Result      CallResponseResult
	Transaction Transaction `json:"transaction"`
}

type CallResponseResult struct {
	Result  bool   `json:"result"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (p *Provider) CallContract(
	ctx context.Context,
	payload ContractCallRequest,
	isTest bool,
) (Transaction, error) {
	req, err := p.newRequest(ctx, http.MethodPost, "/wallet/triggersmartcontract", payload, isTest)
	if err != nil {
		return Transaction{}, errors.Wrap(err, "unable to create request")
	}

	res, err := p.client.Do(req)
	if err != nil {
		return Transaction{}, errors.Wrap(err, "response error")
	}

	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return Transaction{}, errors.Wrap(err, "unable to read response")
	}

	p.logger.Info().
		Interface("request", payload).
		Str("url", req.URL.String()).
		RawJSON("response", body).
		Int("response_code", res.StatusCode).
		Msg("CallContract response")

	if res.StatusCode != http.StatusOK {
		return Transaction{}, errors.Wrapf(ErrResponse, "got %d response code", res.StatusCode)
	}

	var callRes CallResponse
	if err := json.Unmarshal(body, &callRes); err != nil {
		return Transaction{}, errors.Wrap(err, "unmarshal error")
	}

	if !callRes.Result.Result {
		return Transaction{}, errors.Wrapf(ErrResponse, "%s: %s", callRes.Result.Code, callRes.Result.Message)
	}

	return callRes.Transaction, nil
}

// BroadcastResponse. Examples:
//
//	{
//	  "code": "CONTRACT_VALIDATE_ERROR",
//	  "txid": "50d52436163be5c38af269bbdee0a7e258e94cfa9cc05f975e543b98c6994f2e",
//	  "message": "Contract validate error : account [TDVEJbSyEtk8L4htanm8h5mp2sNutRm5ou] does not exist"
//	}
//	{
//	  "code": "DUP_TRANSACTION_ERROR",
//	  "txid": "fbb158948b240f88b188c65f5a623b562fe4d7a9a90eecef4495146acf87a484",
//	  "message": "Dup transaction."
//	}
//	{
//	  "result": true,
//	  "txid": "fbb158948b240f88b188c65f5a623b562fe4d7a9a90eecef4495146acf87a484"
//	}
type BroadcastResponse struct {
	Result            bool   `json:"result"`
	Code              string `json:"code"`
	Message           string `json:"message"`
	TransactionHashID string `json:"txid"`
}

// BroadcastTransaction broadcasts tx and return tx hash id.
func (p *Provider) BroadcastTransaction(ctx context.Context, rawTX []byte, isTest bool) (string, error) {
	req, err := p.newRequest(ctx, http.MethodPost, "/wallet/broadcasttransaction", rawTX, isTest)
	if err != nil {
		return "", errors.Wrap(err, "unable to create request")
	}

	res, err := p.client.Do(req)
	if err != nil {
		return "", errors.Wrap(err, "response error")
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", errors.Wrap(err, "unable to read response")
	}

	defer res.Body.Close()

	p.logger.Info().
		Str("request", string(rawTX)).
		Str("url", req.URL.String()).
		RawJSON("response", body).
		Int("response_code", res.StatusCode).
		Msg("BroadcastTransaction response")

	if res.StatusCode != http.StatusOK {
		return "", errors.Wrapf(ErrResponse, "got %d response code", res.StatusCode)
	}

	var broadcastRes BroadcastResponse
	if err := json.Unmarshal(body, &broadcastRes); err != nil {
		return "", errors.Wrap(err, "unmarshal error")
	}

	if !broadcastRes.Result {
		return "", errors.Wrapf(ErrResponse, "%s: %s", broadcastRes.Code, broadcastRes.Message)
	}

	return broadcastRes.TransactionHashID, nil
}

type TransactionReceipt struct {
	Fee           uint64
	Hash          string
	Sender        string
	Recipient     string
	Confirmations int64
	IsConfirmed   bool
	Success       bool
}

func (p *Provider) GetTransactionReceipt(
	ctx context.Context,
	txID string,
	isTest bool,
) (*TransactionReceipt, error) {
	var (
		tx, info, latestBlock []byte
		mu                    sync.Mutex
		group                 errgroup.Group
	)

	hasData := func(res []byte) bool {
		return gjson.GetBytes(res, "id").Exists() || gjson.GetBytes(res, "txID").Exists()
	}

	group.Go(func() error {
		res, err := p.getTransactionByID(ctx, txID, isTest)
		if err != nil {
			return errors.Wrap(err, "unable to get transaction info")
		}
		if !hasData(res) {
			return ErrNotFound
		}

		mu.Lock()
		tx = res
		mu.Unlock()

		return nil
	})

	group.Go(func() error {
		res, err := p.getTransactionInfoByID(ctx, txID, isTest)
		if err != nil {
			return errors.Wrap(err, "unable to get transaction info")
		}
		if !hasData(res) {
			return ErrNotFound
		}

		mu.Lock()
		info = res
		mu.Unlock()

		return nil
	})

	group.Go(func() error {
		res, err := p.getLatestBlock(ctx, isTest)
		if err != nil {
			return errors.Wrap(err, "unable to get latest block")
		}

		mu.Lock()
		latestBlock = res
		mu.Unlock()

		return nil
	})

	if err := group.Wait(); err != nil {
		return nil, err
	}

	sender := gjson.GetBytes(tx, "raw_data.contract.0.parameter.value.owner_address").String()
	if sender != "" {
		sender = util.TronHexToBase58(sender)
	}

	recipient := gjson.GetBytes(tx, "raw_data.contract.0.parameter.value.to_address").String()
	if recipient == "" {
		recipient = gjson.GetBytes(tx, "raw_data.contract.0.parameter.value.contract_address").String()
	}
	if recipient != "" {
		recipient = util.TronHexToBase58(recipient)
	}

	success := gjson.GetBytes(tx, "ret.0.contractRet").String() == "SUCCESS"
	fee := uint64(gjson.GetBytes(info, "fee").Int())

	txBlockNumber := gjson.GetBytes(info, "blockNumber").Int()
	latestBlockNumber := gjson.GetBytes(latestBlock, "block_header.raw_data.number").Int()
	confirmations := latestBlockNumber - txBlockNumber

	return &TransactionReceipt{
		Fee:           fee,
		Hash:          txID,
		Sender:        sender,
		Recipient:     recipient,
		Confirmations: confirmations,
		IsConfirmed:   confirmations >= confirmationBlocks,
		Success:       success,
	}, nil
}

//	{
//	 "id": "a53122608dbe423c713f9a93b1511d56278df98659aeb12c0c544dc7a88e43c5",
//	 "fee": 36909560,
//	 "blockNumber": 31349959,
//	 "blockTimeStamp": 1676147655000,
//	 "contractResult": [ "..." ],
//	 "contract_address": "41ba221311e9f3ab22c27a2ad48e49f8ca56721da9",
//	 "receipt": {
//	   "energy_usage": 18612,
//	   "energy_fee": 36274560,
//	   "energy_usage_total": 148164,
//	   "net_fee": 635000,
//	 },
//	 "log": [ ... ],
//	 "internal_transactions": [ ... ]
//	}
func (p *Provider) getTransactionInfoByID(ctx context.Context, txID string, isTest bool) ([]byte, error) {
	payload := map[string]string{"value": txID}

	req, err := p.newRequest(ctx, http.MethodPost, "/wallet/gettransactioninfobyid", payload, isTest)
	if err != nil {
		return nil, errors.Wrap(err, "unable to create request")
	}

	res, err := p.client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "response error")
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, errors.Wrap(err, "unable to read response")
	}

	defer res.Body.Close()

	p.logger.Info().
		Str("transaction_id", txID).
		Str("url", req.URL.String()).
		RawJSON("response", body).
		Int("response_code", res.StatusCode).
		Msg("GetTransactionInfoById response")

	return body, nil
}

//	{
//	 "blockID": "...",
//	 "block_header": {
//	   "raw_data": {
//	     "number": 31579273,
//	     "txTrieRoot": "...",
//	     "witness_address": "41711cf6683d28621ae12030fd541b288c61d682cd",
//	     "parentHash": "0000000001e1dc88cbc3e4224d61fd349bcbcae3a7f7bccd5cc616b45fea7cef",
//	     "version": 26,
//	     "timestamp": 1676929908000
//	   },
//	   "witness_signature": "..."
//	 }
//	}
func (p *Provider) getLatestBlock(ctx context.Context, isTest bool) ([]byte, error) {
	req, err := p.newRequest(ctx, http.MethodGet, "/walletsolidity/getnowblock", nil, isTest)
	if err != nil {
		return nil, errors.Wrap(err, "unable to create request")
	}

	res, err := p.client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "response error")
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, errors.Wrap(err, "unable to read response")
	}

	defer res.Body.Close()

	p.logger.Info().
		Str("url", req.URL.String()).
		RawJSON("response", body).
		Int("response_code", res.StatusCode).
		Msg("GetLatestBlockNumber response")

	return body, nil
}

//	{
//	 "ret": [
//	   { "contractRet": "SUCCESS" }
//	 ],
//	 "signature": [ "..." ],
//	 "txID": "0471ef93ae986f8c73900787e429c570d96c161b7b25c59271083e80b1d460fc",
//	 "raw_data": {
//	   "contract": [
//	     {
//	       "parameter": {
//	         "value": {
//	           "amount": 5000000000,
//	           "owner_address": "41b3dcf27c251da9363f1a4888257c16676cf54edf",
//	           "to_address": "4199409c7014a738224159a8d3e12cc90163ce6db2"
//	         },
//	         "type_url": "type.googleapis.com/protocol.TransferContract"
//	       },
//	       "type": "TransferContract"
//	     }
//	   ],
//	   "ref_block_bytes": "6828",
//	   "ref_block_hash": "9bdfb56481721d3e",
//	   "expiration": 1676157717000,
//	   "timestamp": 1676157659717
//	 },
//	 "raw_data_hex": "..."
//	}
func (p *Provider) getTransactionByID(ctx context.Context, txID string, isTest bool) ([]byte, error) {
	payload := map[string]string{"value": txID}

	req, err := p.newRequest(ctx, http.MethodPost, "/wallet/gettransactionbyid", payload, isTest)
	if err != nil {
		return nil, errors.Wrap(err, "unable to create request")
	}

	res, err := p.client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "response error")
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, errors.Wrap(err, "unable to read response")
	}

	defer res.Body.Close()

	p.logger.Info().
		Str("transaction_id", txID).
		Str("url", req.URL.String()).
		RawJSON("response", body).
		Int("response_code", res.StatusCode).
		Msg("GetTransactionById response")

	return body, nil
}

//nolint:unparam
func (p *Provider) newRequest(
	ctx context.Context, method string, path string, payload any, isTest bool,
) (*http.Request, error) {
	var reqBody bytes.Buffer
	if payload != nil {
		if payloadBytes, ok := payload.([]byte); ok {
			reqBody.Write(payloadBytes)
		} else {
			buffer, err := json.Marshal(payload)
			if err != nil {
				return nil, errors.Wrap(err, "unable to marshal payload")
			}

			if _, err := reqBody.Write(buffer); err != nil {
				return nil, errors.Wrap(err, "unable to write buffer")
			}
		}
	}

	var url string
	if isTest {
		url = p.config.TestnetBaseURL + path
	} else {
		url = p.config.MainnetBaseURL + path
	}

	req, err := http.NewRequestWithContext(ctx, method, url, &reqBody)
	if err != nil {
		return nil, errors.Wrap(err, "unable to create request")
	}

	if !isTest {
		req.Header.Set(headerAPIKey, p.config.APIKey)
	}

	if payload != nil {
		req.Header.Set("content-type", "application/json")
	}

	return req, nil
}
