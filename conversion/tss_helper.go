package conversion

import (
	"errors"
	"math/rand"
	"sort"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	atypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/tendermint/tendermint/crypto/secp256k1"
)

// GetRandomPubKey for test
func GetRandomPubKey() string {
	_, pubKey, _ := atypes.KeyTestPubAddr()
	bech32PubKey, _ := sdk.Bech32ifyPubKey(sdk.Bech32PubKeyTypeAccPub, pubKey)
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
	for _, n := range in {
		freq[n]++
	}
	max := 0
	var values []string
	for v, f := range freq {
		if f > max {
			max = f
			values = make([]string, 1)
			values[0] = v
		} else {
			if f == max {
				values = append(values, v)
			}
		}
	}
	sort.Slice(values, func(i, j int) bool {
		return values[i] > values[j]
	})

	return values[0], max, nil
}

func GetP2PProtocol(ver string, protos []string) (protocol.ID, error) {
	if len(protos) == 0 {
		return "", errors.New("empty protocols array")
	}

	a := strings.Split(ver, "-")
	if len(a) == 0 {
		return "", errors.New("invalid input version")
	}
	verHead := strings.TrimPrefix(a[0], "gg")

	for _, el := range protos {
		a := strings.Split(el, "/")
		if len(a) < 1 {
			return "", errors.New("fail to get the supported version")
		}
		proto := a[len(a)-1]
		if verHead == proto {
			return protocol.ID(el), nil
		}
	}

	return "", errors.New("p2p protocol not found")
}
