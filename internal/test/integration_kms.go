package test

import (
	cryptorand "crypto/rand"
	"testing"

	"github.com/oxygenpay/oxygen/internal/db/connection/bolt"
	"github.com/oxygenpay/oxygen/internal/kms/wallet"
	"github.com/oxygenpay/oxygen/internal/provider/trongrid"
	"github.com/rs/zerolog"
	"go.etcd.io/bbolt"
)

type KMS struct {
	Bolt       *bbolt.DB
	Repository *wallet.Repository
	Service    *wallet.Service
}

func setupKMS(t *testing.T, trongridProvider *trongrid.Provider, logger *zerolog.Logger) *KMS {
	dataSource := t.TempDir() + "/kms.test.db"

	conn, err := bolt.Open(bolt.Config{DataSource: dataSource}, logger)
	if err != nil {
		t.Fatalf("unable to connect to bolt db: %s", err)
	}

	if err := conn.LoadBuckets(); err != nil {
		t.Fatalf("unable to load kms bolt db buckets: %s", err)
	}

	walletGenerator :=
		wallet.NewGenerator().
			AddProvider(&wallet.EthProvider{Blockchain: wallet.ETH, CryptoReader: cryptorand.Reader}).
			AddProvider(&wallet.EthProvider{Blockchain: wallet.MATIC, CryptoReader: cryptorand.Reader}).
			AddProvider(&wallet.EthProvider{Blockchain: wallet.BSC, CryptoReader: cryptorand.Reader}).
			AddProvider(&wallet.BitcoinProvider{Blockchain: wallet.BTC, CryptoReader: cryptorand.Reader}).
			AddProvider(&wallet.TronProvider{
				Blockchain:   wallet.TRON,
				CryptoReader: cryptorand.Reader,
				Trongrid:     trongridProvider,
			})

	repo := wallet.NewRepository(conn.DB())

	return &KMS{
		Bolt:       conn.DB(),
		Repository: repo,
		Service:    wallet.New(repo, walletGenerator, logger),
	}
}
