package test

import (
	"time"

	"github.com/google/uuid"
	"github.com/oxygenpay/oxygen/internal/util"
	kmswallet "github.com/oxygenpay/oxygen/pkg/api-kms/v1/client/wallet"
	kmsmodel "github.com/oxygenpay/oxygen/pkg/api-kms/v1/model"
	"github.com/stretchr/testify/mock"
)

func (i *IntegrationTest) SetupCreateWallet(blockchain, address, pubKey string) {
	bc := kmsmodel.Blockchain(blockchain)
	w := &kmsmodel.Wallet{
		ID:            uuid.New().String(),
		CreatedAtUnix: time.Now().Unix(),
		Blockchain:    bc,
		Address:       address,
		PublicKey:     pubKey,
	}

	matcher := mock.MatchedBy(func(r *kmswallet.CreateWalletParams) bool {
		return r.Data.Blockchain == bc
	})

	i.Providers.KMS.
		On("CreateWallet", matcher).
		Return(&kmswallet.CreateWalletCreated{Payload: w}, nil).
		Once()
}

const RandomAddress = "random"

func (i *IntegrationTest) SetupCreateWalletWithSubscription(blockchain, address, pubKey string) {
	if address == RandomAddress {
		address = "0x123-" + util.Strings.Random(6)
	}

	i.SetupCreateWallet(blockchain, address, pubKey)

	id := util.Strings.Random(6)

	i.Providers.TatumMock.SetupSubscription(blockchain, address, false, "testnet_"+id)
	i.Providers.TatumMock.SetupSubscription(blockchain, address, true, "mainnet_"+id)
}

func (i *IntegrationTest) SetupCreateEthereumTransaction(
	walletID uuid.UUID,
	input kmsmodel.CreateEthereumTransactionRequest,
	rawTx string,
) {
	req := &kmswallet.CreateEthereumTransactionParams{
		Data:     &input,
		WalletID: walletID.String(),
	}

	res := &kmswallet.CreateEthereumTransactionCreated{
		Payload: &kmsmodel.EthereumTransaction{
			RawTransaction: rawTx,
		},
	}

	i.Providers.KMS.On("CreateEthereumTransaction", req).Return(res, nil)
}

func (i *IntegrationTest) SetupCreateEthereumTransactionWildcard(rawTx string) {
	res := &kmswallet.CreateEthereumTransactionCreated{
		Payload: &kmsmodel.EthereumTransaction{
			RawTransaction: rawTx,
		},
	}

	i.Providers.KMS.On("CreateEthereumTransaction", mock.Anything).Return(res, nil)
}

func (i *IntegrationTest) SetupCreateMaticTransaction(
	walletID uuid.UUID,
	input kmsmodel.CreateMaticTransactionRequest,
	rawTx string,
) {
	req := &kmswallet.CreateMaticTransactionParams{
		Data:     &input,
		WalletID: walletID.String(),
	}

	res := &kmswallet.CreateMaticTransactionCreated{
		Payload: &kmsmodel.MaticTransaction{
			RawTransaction: rawTx,
		},
	}

	i.Providers.KMS.On("CreateMaticTransaction", req).Return(res, nil)
}

func (i *IntegrationTest) SetupCreateMaticTransactionWildcard(rawTx string) {
	res := &kmswallet.CreateMaticTransactionCreated{
		Payload: &kmsmodel.MaticTransaction{
			RawTransaction: rawTx,
		},
	}

	i.Providers.KMS.On("CreateMaticTransaction", mock.Anything).Return(res, nil)
}

func (i *IntegrationTest) SetupCreateBSCTransaction(
	walletID uuid.UUID,
	input kmsmodel.CreateBSCTransactionRequest,
	rawTx string,
) {
	req := &kmswallet.CreateBSCTransactionParams{
		Data:     &input,
		WalletID: walletID.String(),
	}

	res := &kmswallet.CreateBSCTransactionCreated{
		Payload: &kmsmodel.BSCTransaction{
			RawTransaction: rawTx,
		},
	}

	i.Providers.KMS.On("CreateBSCTransaction", req).Return(res, nil)
}

func (i *IntegrationTest) SetupCreateBSCTransactionWildcard(rawTx string) {
	res := &kmswallet.CreateBSCTransactionCreated{
		Payload: &kmsmodel.BSCTransaction{
			RawTransaction: rawTx,
		},
	}

	i.Providers.KMS.On("CreateBSCTransaction", mock.Anything).Return(res, nil)
}

func (i *IntegrationTest) SetupCreateTronTransaction(
	walletID uuid.UUID,
	input kmsmodel.CreateTronTransactionRequest,
	rawTx string,
) {
	req := &kmswallet.CreateTronTransactionParams{
		WalletID: walletID.String(),
		Data:     &input,
	}

	res := &kmswallet.CreateTronTransactionCreated{
		Payload: &kmsmodel.TronTransaction{
			RawDataHex: rawTx,
		},
	}

	i.Providers.KMS.On("CreateTronTransaction", req).Return(res, nil)
}

func (i *IntegrationTest) SetupCreateTronTransactionWildcard(rawTx string) {
	res := &kmswallet.CreateTronTransactionCreated{
		Payload: &kmsmodel.TronTransaction{
			RawDataHex: rawTx,
		},
	}

	i.Providers.KMS.On("CreateTronTransaction", mock.Anything).Return(res, nil)
}
