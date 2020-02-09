package tss

import (
	"errors"

	"github.com/binance-chain/tss-lib/crypto"
	"github.com/btcsuite/btcd/btcec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/tendermint/tendermint/crypto/secp256k1"
)

func getThorPubKey(pubKeyPoint *crypto.ECPoint) (string, error) {
	if pubKeyPoint == nil {
		return "", errors.New("invalid points")
	}
	tssPubKey := btcec.PublicKey{
		Curve: btcec.S256(),
		X:     pubKeyPoint.X(),
		Y:     pubKeyPoint.Y(),
	}
	var pubKeyCompressed secp256k1.PubKeySecp256k1
	copy(pubKeyCompressed[:], tssPubKey.SerializeCompressed())
	pubKey, err := sdk.Bech32ifyAccPub(pubKeyCompressed)
	return pubKey, err
}
