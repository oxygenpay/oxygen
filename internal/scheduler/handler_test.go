package scheduler_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgtype"
	"github.com/oxygenpay/oxygen/internal/db/repository"
	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/oxygenpay/oxygen/internal/scheduler"
	"github.com/oxygenpay/oxygen/internal/service/payment"
	"github.com/oxygenpay/oxygen/internal/service/processing"
	"github.com/oxygenpay/oxygen/internal/service/transaction"
	"github.com/oxygenpay/oxygen/internal/service/wallet"
	"github.com/oxygenpay/oxygen/internal/test"
	"github.com/oxygenpay/oxygen/internal/test/mock"
	"github.com/oxygenpay/oxygen/internal/util"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type TestContext struct {
	*test.IntegrationTest

	Context        context.Context
	ProcessingMock *mock.ProcessingProxyMock
	Scheduler      *scheduler.Handler
}

func setup(t *testing.T) *TestContext {
	tc := test.NewIntegrationTest(t)

	ctx := context.WithValue(context.Background(), scheduler.ContextJobID{}, "job-abc")
	ctx = tc.Logger.WithContext(ctx)

	processingMock := mock.NewProcessingProxyMock(t, tc.Services.Processing)

	return &TestContext{
		IntegrationTest: tc,

		Context:        ctx,
		ProcessingMock: processingMock,
		Scheduler: scheduler.New(
			tc.Services.Payment,
			tc.Services.Blockchain,
			tc.Services.Wallet,
			processingMock,
			tc.Services.Transaction,
			tc.Services.JobLogger,
		),
	}
}

func TestHandler_CheckIncomingTransactionsProgress(t *testing.T) {
	// ARRANGE
	tc := setup(t)

	// Given merchant
	mt, _ := tc.Must.CreateMerchant(t, 1)

	// Given outbound wallet
	inboundWallet := tc.Must.CreateWallet(t, "ETH", "0x123", "0x123-pub-key", wallet.TypeInbound)
	outboundWallet := tc.Must.CreateWallet(t, "ETH", "0x1234", "0x1234-pub-key", wallet.TypeOutbound)

	// Given several transactions
	asIncoming := func(p *transaction.CreateTransaction) {
		p.Type = transaction.TypeIncoming
		p.RecipientWallet = inboundWallet
	}

	asInternal := func(p *transaction.CreateTransaction) {
		p.Type = transaction.TypeInternal
		p.EntityID = 0
		p.SenderWallet = inboundWallet
		p.RecipientWallet = outboundWallet
	}

	// Given 2 incoming 'in progress' txs
	tx1 := tc.Must.CreateTransaction(t, mt.ID, asIncoming)
	_, err := tc.Repository.UpdateTransaction(tc.Context, repository.UpdateTransactionParams{
		MerchantID: mt.ID,
		ID:         tx1.ID,
		Status:     string(transaction.StatusInProgress),
		UpdatedAt:  time.Now(),
		FactAmount: pgtype.Numeric{Status: pgtype.Null},
		NetworkFee: pgtype.Numeric{Status: pgtype.Null},
		Metadata:   pgtype.JSONB{Status: pgtype.Null},
	})
	require.NoError(t, err)

	tx2 := tc.Must.CreateTransaction(t, mt.ID, asIncoming)
	_, err = tc.Repository.UpdateTransaction(tc.Context, repository.UpdateTransactionParams{
		MerchantID: mt.ID,
		ID:         tx2.ID,
		Status:     string(transaction.StatusInProgressInvalid),
		UpdatedAt:  time.Now(),
		FactAmount: pgtype.Numeric{Status: pgtype.Null},
		NetworkFee: pgtype.Numeric{Status: pgtype.Null},
		Metadata:   pgtype.JSONB{Status: pgtype.Null},
	})
	require.NoError(t, err)

	// And 1 incoming 'pending' & 2 internal txs
	tc.Must.CreateTransaction(t, mt.ID, asIncoming)
	tc.Must.CreateTransaction(t, 0, asInternal)
	tc.Must.CreateTransaction(t, 0, asInternal)

	// And expected mock
	tc.ProcessingMock.SetupBatchCheckIncomingTransactions([]int64{tx1.ID, tx2.ID}, nil)

	// ACT
	err = tc.Scheduler.CheckIncomingTransactionsProgress(tc.Context)

	// ASSERT
	assert.NoError(t, err)
}

//nolint:funlen
func TestScheduler(t *testing.T) {
	t.Run("PerformInternalWalletTransfer", func(t *testing.T) {
		// Assume that DB is empty and the system need to create outbound wallets for each blockchain
		t.Run("Empty state", func(t *testing.T) {
			// ARRANGE
			tc := setup(t)

			// Given mocked responses from tatum KMS
			allBlockchains := tc.Services.Blockchain.ListSupportedBlockchains()
			for _, bc := range allBlockchains {
				tc.SetupCreateWalletWithSubscription(bc.String(), "abc-123", "pub-key-123")
			}

			// And mocked processing method
			tc.ProcessingMock.SetupBatchCreateInternalTransfers(nil, nil, nil)

			// ACT
			err := tc.Scheduler.PerformInternalWalletTransfer(tc.Context)

			// ASSERT
			assert.NoError(t, err)

			// Check that all OUTBOUND wallets are in place with empty balances
			for _, bc := range allBlockchains {
				//nolint:govet
				wallets, err := tc.Repository.PaginateWalletsByID(tc.Context, repository.PaginateWalletsByIDParams{
					Limit:              1,
					Type:               repository.StringToNullable(string(wallet.TypeOutbound)),
					Blockchain:         bc.String(),
					FilterByType:       true,
					FilterByBlockchain: true,
				})

				assert.NoError(t, err)
				assert.Len(t, wallets, 1)

				w := wallets[0]

				// Check that wallet is subscribed to Tatum
				assert.NotEmpty(t, w.TatumTestnetSubscriptionID.String)
				assert.NotEmpty(t, w.TatumMainnetSubscriptionID.String)

				// Check that empty balance records exist
				currencies := tc.Services.Blockchain.ListBlockchainCurrencies(bc)
				currenciesByTicker := util.KeyFunc(currencies, func(c money.CryptoCurrency) string {
					return c.Ticker
				})

				balances, err := tc.Services.Wallet.ListBalances(tc.Context, wallet.EntityTypeWallet, w.ID, false)
				assert.NoError(t, err)
				for _, b := range balances {
					c, exists := currenciesByTicker[b.Currency]
					assert.True(t, exists)
					assert.Equal(t, c.Decimals, b.Amount.Decimals())
					assert.Equal(t, c.Type, b.CurrencyType)
				}
			}

			// Check job logs
			// "fetched inbound wallets" + "matched inbound balances" + "created internal transactions"
			tc.AssertTableRows(t, "job_logs", 3)
			tc.AssertTableRows(t, "wallets", 4)

			// Check that duplicate outbound wallet duplicate creation is not possible
			tc.SetupCreateWalletWithSubscription("ETH", "0x2222", "0x123-pub-key")
			_, err = tc.Services.Wallet.Create(tc.Context, "ETH", wallet.TypeOutbound)
			require.Error(t, err)

			t.Run("Match two balances", func(t *testing.T) {
				//nolint:unparam
				makeBalance := func(w *wallet.Wallet, c money.CryptoCurrency, value string, usdRate float64, isTest bool) *wallet.Balance {
					b := tc.Must.CreateBalance(t, wallet.EntityTypeWallet, w.ID, test.WithBalanceFromCurrency(c, value, isTest))
					tc.Providers.TatumMock.SetupRates(c.Ticker, money.USD, usdRate)

					return b
				}

				eth, err := tc.Services.Blockchain.GetCurrencyByTicker("ETH")
				require.NoError(t, err)

				ethUSDT, err := tc.Services.Blockchain.GetCurrencyByTicker("ETH_USDT")
				require.NoError(t, err)

				matic, err := tc.Services.Blockchain.GetCurrencyByTicker("MATIC")
				require.NoError(t, err)

				maticUSDT, err := tc.Services.Blockchain.GetCurrencyByTicker("MATIC_USDT")
				require.NoError(t, err)

				// ARRANGE
				// Clear previous job logs
				tc.Clear.Table(t, "job_logs")

				// Given ETH & MATIC wallet with balances
				w1 := tc.Must.CreateWallet(t, "ETH", "0x123", "pub-key", wallet.TypeInbound)
				w2 := tc.Must.CreateWallet(t, "MATIC", "0x123", "pub-key", wallet.TypeInbound)

				makeBalance(w1, eth, "123000", 1000, false)
				makeBalance(w2, matic, "123000", 10, false)

				// these balances should match
				b3 := makeBalance(w1, ethUSDT, "120_000000", 1, false)
				b4 := makeBalance(w2, maticUSDT, "130_000000", 1, false)

				// And mocked processing method response
				tc.ProcessingMock.SetupBatchCreateInternalTransfers(
					[]*wallet.Balance{b3, b4},
					&processing.TransferResult{
						CreatedTransactions: []*transaction.Transaction{{ID: 123}, {ID: 456}},
					},
					nil,
				)

				// ACT
				err = tc.Scheduler.PerformInternalWalletTransfer(tc.Context)

				// ASSERT
				assert.NoError(t, err)

				// Check job logs
				// "fetched inbound wallets" + "matched inbound balances" + "created internal transactions"
				tc.AssertTableRows(t, "job_logs", 3)
			})
		})
	})

	t.Run("CheckInternalTransferProgress", func(t *testing.T) {
		// ARRANGE
		tc := setup(t)

		// Given merchant
		mt, _ := tc.Must.CreateMerchant(t, 1)

		// Given outbound wallet
		inboundWallet := tc.Must.CreateWallet(t, "ETH", "0x123", "0x123-pub-key", wallet.TypeInbound)
		outboundWallet := tc.Must.CreateWallet(t, "ETH", "0x1234", "0x1234-pub-key", wallet.TypeOutbound)

		// Given several transactions
		asIncoming := func(p *transaction.CreateTransaction) {
			p.Type = transaction.TypeIncoming
			p.RecipientWallet = inboundWallet
		}

		asInternal := func(p *transaction.CreateTransaction) {
			p.Type = transaction.TypeInternal
			p.EntityID = 0
			p.SenderWallet = inboundWallet
			p.RecipientWallet = outboundWallet
		}

		_ = tc.Must.CreateTransaction(t, mt.ID, asIncoming)

		tx1 := tc.Must.CreateTransaction(t, 0, asInternal)
		tx2 := tc.Must.CreateTransaction(t, 0, asInternal)

		// And 'in progress' tx
		tx3 := tc.Must.CreateTransaction(t, 0, asInternal)
		_, err := tc.Repository.UpdateTransaction(tc.Context, repository.UpdateTransactionParams{
			MerchantID: 0,
			ID:         tx3.ID,
			Status:     string(transaction.StatusInProgress),
			UpdatedAt:  time.Now(),
			FactAmount: pgtype.Numeric{Status: pgtype.Null},
			NetworkFee: pgtype.Numeric{Status: pgtype.Null},
			Metadata:   pgtype.JSONB{Status: pgtype.Null},
		})
		require.NoError(t, err)

		// And 'completed' tx
		tx4 := tc.Must.CreateTransaction(t, 0, asInternal)
		_, err = tc.Repository.UpdateTransaction(tc.Context, repository.UpdateTransactionParams{
			MerchantID: 0,
			ID:         tx4.ID,
			Status:     string(transaction.StatusCompleted),
			UpdatedAt:  time.Now(),
			FactAmount: pgtype.Numeric{Status: pgtype.Null},
			NetworkFee: pgtype.Numeric{Status: pgtype.Null},
			Metadata:   pgtype.JSONB{Status: pgtype.Null},
		})
		require.NoError(t, err)

		// And expected mock
		tc.ProcessingMock.SetupBatchCheckInternalTransfers([]int64{tx1.ID, tx2.ID, tx3.ID}, nil)

		// ACT
		err = tc.Scheduler.CheckInternalTransferProgress(tc.Context)

		// ASSERT
		assert.NoError(t, err)
	})

	t.Run("PerformWithdrawalsCreation", func(t *testing.T) {
		// ARRANGE
		tc := setup(t)

		createWithdrawal := func(merchantID int64, fns ...func(*repository.CreatePaymentParams)) *payment.Payment {
			params := repository.CreatePaymentParams{
				CreatedAt:         time.Now(),
				UpdatedAt:         time.Now(),
				PublicID:          uuid.New(),
				MerchantOrderUuid: uuid.New(),
				Type:              string(payment.TypeWithdrawal),
				Status:            string(payment.StatusPending),
				MerchantID:        merchantID,

				// custom defined
				Price:    pgtype.Numeric{},
				Decimals: 0,
				Currency: "",
				IsTest:   false,
				Metadata: make(payment.Metadata).ToJSONB(),
			}

			for _, fn := range fns {
				fn(&params)
			}

			entry, err := tc.Repository.CreatePayment(tc.Context, params)
			require.NoError(t, err)

			p, err := tc.Services.Payment.GetByID(tc.Context, entry.MerchantID, entry.ID)
			require.NoError(t, err)

			return p
		}

		// Given 2 merchant
		mt1, _ := tc.Must.CreateMerchant(t, 1)
		mt2, _ := tc.Must.CreateMerchant(t, 1)

		// With several payments
		tc.CreateSamplePayment(t, mt1.ID)
		tc.CreateSamplePayment(t, mt1.ID)
		tc.CreateSamplePayment(t, mt2.ID)
		tc.CreateSamplePayment(t, mt2.ID)

		// And pending withdrawals
		eth := lo.Must(tc.Services.Blockchain.GetCurrencyByTicker("ETH"))
		ethWithdrawal := func(p *repository.CreatePaymentParams) {
			p.Price = repository.MoneyToNumeric(money.MustCryptoFromRaw(eth.Ticker, "1", eth.Decimals))
			p.Decimals = int32(eth.Decimals)
			p.Currency = eth.Ticker
		}

		p1 := createWithdrawal(mt1.ID, ethWithdrawal)
		p2 := createWithdrawal(mt2.ID, ethWithdrawal)

		// And "in progress" withdrawal that should be skipped
		createWithdrawal(mt2.ID, ethWithdrawal, func(p *repository.CreatePaymentParams) {
			p.Status = string(payment.StatusInProgress)
		})

		// And mocked responses from tatum KMS for ensuring outbound wallets
		allBlockchains := tc.Services.Blockchain.ListSupportedBlockchains()
		for _, bc := range allBlockchains {
			tc.SetupCreateWalletWithSubscription(bc.String(), "abc-123", "pub-key-123")
		}

		// And mocked response from processing service
		tc.ProcessingMock.SetupBatchCreateWithdrawals(
			[]int64{p1.ID, p2.ID},
			&processing.TransferResult{},
			nil,
		)

		// ACT
		err := tc.Scheduler.PerformWithdrawalsCreation(tc.Context)

		// ASSERT
		assert.NoError(t, err)
	})

	t.Run("CheckWithdrawalsProgress", func(t *testing.T) {
		// ARRANGE
		tc := setup(t)

		// Given a merchant
		mt, _ := tc.Must.CreateMerchant(t, 1)

		// With a wallet
		wt := tc.Must.CreateWallet(t, "ETH", "0x123", "0x123-pub-key", wallet.TypeOutbound)

		// Given several transactions
		incoming := func(p *transaction.CreateTransaction) {
			p.RecipientWallet = wt
		}

		withdrawal := func(p *transaction.CreateTransaction) {
			p.Type = transaction.TypeWithdrawal
			p.SenderWallet = wt
		}

		tc.Must.CreateTransaction(t, 1, incoming)
		tx1 := tc.Must.CreateTransaction(t, mt.ID, withdrawal)
		tx2 := tc.Must.CreateTransaction(t, mt.ID, withdrawal)

		// And 'in progress' tx
		tx3 := tc.Must.CreateTransaction(t, mt.ID, withdrawal)
		_, err := tc.Repository.UpdateTransaction(tc.Context, repository.UpdateTransactionParams{
			MerchantID: mt.ID,
			ID:         tx3.ID,
			Status:     string(transaction.StatusInProgress),
			UpdatedAt:  time.Now(),
			FactAmount: pgtype.Numeric{Status: pgtype.Null},
			NetworkFee: pgtype.Numeric{Status: pgtype.Null},
			Metadata:   pgtype.JSONB{Status: pgtype.Null},
		})
		require.NoError(t, err)

		// and expected mock
		tc.ProcessingMock.SetupBatchCheckWithdrawals([]int64{tx1.ID, tx2.ID, tx3.ID}, nil)

		// ACT
		err = tc.Scheduler.CheckWithdrawalsProgress(tc.Context)

		// ASSERT
		assert.NoError(t, err)
	})

	t.Run("CancelExpiredPayments", func(t *testing.T) {
		// ARRANGE
		tc := setup(t)

		// Given a merchant
		mt, _ := tc.Must.CreateMerchant(t, 1)

		setExpiration := func(pt *payment.Payment, status payment.Status, expiresAt time.Time) {
			_, err := tc.Repository.UpdatePayment(tc.Context, repository.UpdatePaymentParams{
				ID:           pt.ID,
				MerchantID:   pt.MerchantID,
				Status:       string(status),
				UpdatedAt:    time.Now(),
				ExpiresAt:    repository.TimeToNullable(expiresAt),
				SetExpiresAt: true,
			})
			require.NoError(t, err)
		}

		alterCreatedAt := func(dur time.Duration) func(*repository.CreatePaymentParams) {
			return func(create *repository.CreatePaymentParams) {
				create.CreatedAt = time.Now().Add(dur)
			}
		}

		// With several payments
		pt1 := tc.CreateSamplePayment(t, mt.ID)
		pt2 := tc.CreateSamplePayment(t, mt.ID)
		pt3 := tc.CreateSamplePayment(t, mt.ID)
		pt4 := tc.CreateSamplePayment(t, mt.ID)

		// And some payments should be expired
		setExpiration(pt1, payment.StatusPending, time.Now().Add(payment.ExpirationPeriodForLocked))
		setExpiration(pt2, payment.StatusPending, time.Now()) // should expire
		setExpiration(pt3, payment.StatusLocked, time.Now().Add(payment.ExpirationPeriodForLocked/2))
		setExpiration(pt4, payment.StatusLocked, time.Now().Add(-time.Minute)) // should expire

		// And payments that were created a long time ago, but didn't have any interaction from a user.
		// pt5Raw shouldn't be included in the batch
		_ = tc.CreateRawPayment(t, mt.ID, alterCreatedAt(-payment.ExpirationPeriodForNotLocked+time.Minute))

		// should be included in the batch
		pt6Raw := tc.CreateRawPayment(t, mt.ID, alterCreatedAt(-payment.ExpirationPeriodForNotLocked))

		// And expected processing service call
		tc.ProcessingMock.SetupBatchExpirePayments([]int64{pt2.ID, pt4.ID, pt6Raw.ID}, nil)

		// ACT
		err := tc.Scheduler.CancelExpiredPayments(tc.Context)

		// ASSERT
		assert.NoError(t, err)
	})
}
