package util

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/btcsuite/btcutil/base58"
)

// input: 41b35b60a4572e473e492ee35f0750f95c682e081c
// output: TSKZRR9egK9YSXGdbVQGrVoBVc18AYpEBz
func TronHexToBase58(address string) string {
	addressHex, _ := hex.DecodeString(address)

	// https://gist.github.com/motopig/c680f53897429fd15f5b3ca9aa6f6ed2
	// https://github.com/tronprotocol/tronweb/blob/master/src/utils/accounts.js#L15
	// Honestly, atm I don't understand this code. Let's figure it out later :)
	hash1 := SHA256(SHA256(addressHex))
	secret := hash1[:4]
	addressHex = append(addressHex, secret...)

	return base58.Encode(addressHex)
}

func SHA256(s []byte) []byte {
	h := sha256.New()
	h.Write(s)
	bs := h.Sum(nil)
	return bs
}
