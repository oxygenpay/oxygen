package wallet_test

import (
	cryptorand "crypto/rand"
	"testing"

	"github.com/oxygenpay/oxygen/internal/kms/wallet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wemeetagain/go-hdwallet"
)

func TestBitcoinProvider_Generate(t *testing.T) {
	const (
		mockAddress    = "1EF8umvFfsxuMK3QiubmrcGDjcNz2UJvgr"
		mockPubKey     = "xpub661MyMwAqRbcGRWWcY4qYw2QpdFnfQz31BLdkDqXvac9Cp4zk5J4NEqssAd3CEfSSUEsh183dN93xzPhbnprMKaq9E5BVkagZLFnqTVPBCy"
		mockPrivateKey = "xprv9s21ZrQH143K3wS3WWXqBo5gGbRJFxGBdxR2wqRvNF5AL1jrCXyopSXQ1tduFqKjJq4CbP3dPMH48JtKhMtm7zNLytntFN8NRsGaYJwJ3Ku"
	)

	p := &wallet.BitcoinProvider{
		Blockchain:   wallet.BTC,
		CryptoReader: &fakeReader{},
	}

	t.Run("Mock_GenerationSuccessful", func(t *testing.T) {
		w := p.Generate()

		assert.Equal(t, w.Address, mockAddress)
		assert.Equal(t, w.PublicKey, mockPubKey)
		assert.Equal(t, w.PrivateKey, mockPrivateKey)
	})

	t.Run("Mock_PrivateKeyAsStringToPublicKey", func(t *testing.T) {
		w := p.Generate()

		key, err := hdwallet.StringWallet(w.PrivateKey)
		require.NoError(t, err)

		publicKey := key.Pub().String()
		assert.Equal(t, publicKey, w.PublicKey)
	})

	t.Run("Mock_PrivateKeyAsStringToAddress", func(t *testing.T) {
		w := p.Generate()

		key, err := hdwallet.StringWallet(w.PrivateKey)
		require.NoError(t, err)

		address := key.Pub().Address()
		assert.Equal(t, address, w.Address)
	})

	t.Run("Real_GenerationSuccessful", func(t *testing.T) {
		p := &wallet.BitcoinProvider{
			Blockchain:   wallet.BTC,
			CryptoReader: cryptorand.Reader,
		}

		w := p.Generate()

		key, err := hdwallet.StringWallet(w.PrivateKey)
		require.NoError(t, err)

		publicKey := key.Pub().String()
		address := key.Pub().Address()

		assert.Equal(t, publicKey, w.PublicKey)
		assert.Equal(t, address, w.Address)
	})
}

func TestBitcoinProvider_ValidateAddress(t *testing.T) {
	p := &wallet.BitcoinProvider{}

	for _, tc := range []struct {
		addr          string
		expectInvalid bool
	}{
		{addr: "bc1qar0srrr7xfkvy5l643lydnw9re59gtzzwf5mdq"},
		{addr: "bc1ql7c7u74ht6j02wt56csd43wfsnv5949xqwkx7h"},
		{addr: "37fiwTokZXVyao1iugda5cGAmkzfYAwNYW"},
		{addr: "1LQoWist8KkaUXSPKZHNvEyfrEkPHzSsCd"},
		{addr: "1FeexV6bAHb8ybZjqQMjJrcCrHGW9sb6uF"},
		{addr: "2FeexV6bAHb8ybZjqQMjJrcCrHGW9sb6uF", expectInvalid: true},
		{addr: "1FeexV6bAHb8ybZjqQMjJrcCrHGW9sb6uF_", expectInvalid: true},
		{addr: "1FeexV6bAHb8ybZjqQMjJrcCrHGW9sb6uF_", expectInvalid: true},
		{addr: "1FeexV6bAHb8ybZjqQMjJH", expectInvalid: true},
		{addr: "wtf", expectInvalid: true},
	} {
		t.Run(tc.addr, func(t *testing.T) {
			assert.Equal(t, !tc.expectInvalid, p.ValidateAddress(tc.addr))
		})
	}
}
