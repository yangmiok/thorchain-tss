package conversion

import (
	"errors"
	"math/rand"
	"sort"
	"strconv"

	sdk "github.com/cosmos/cosmos-sdk/types"
	atypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/tendermint/tendermint/crypto/secp256k1"
)

// GetRandomPubKey for test
func GetRandomPubKey() string {
	_, pubKey, _ := atypes.KeyTestPubAddr()
	bech32PubKey, _ := sdk.Bech32ifyAccPub(pubKey)
	return bech32PubKey
}

// GetRandomPeerID for test
func GetRandomPeerID() peer.ID {
	_, pubKey, _ := atypes.KeyTestPubAddr()
	peerID, _ := GetPeerIDFromSecp256PubKey(pubKey.(secp256k1.PubKeySecp256k1))
	return peerID
}

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
)

func RandStringBytesMask(n int) string {
	b := make([]byte, n)
	for i := 0; i < n; {
		if idx := int(rand.Int63() & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i++
		}
	}
	return string(b)
}

func GetHighestFreq(in map[string]string) (string, int, error) {
	if len(in) == 0 {
		return "", 0, errors.New("empty input")
	}
	freq := make(map[string]int, len(in))
	hashPeerMap := make(map[string]string, len(in))
	for peer, n := range in {
		freq[n]++
		hashPeerMap[n] = peer
	}

	sFreq := make([][2]string, 0, len(freq))
	for n, f := range freq {
		sFreq = append(sFreq, [2]string{n, strconv.FormatInt(int64(f), 10)})
	}
	sort.Slice(sFreq, func(i, j int) bool {
		if sFreq[i][1] > sFreq[j][1] {
			return true
		} else {
			return false
		}
	},
	)
	freqInt, err := strconv.Atoi(sFreq[0][1])
	if err != nil {
		return "", 0, err
	}
	return sFreq[0][0], freqInt, nil
}
