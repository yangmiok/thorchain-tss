package conversion

import (
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"strings"

	semver "github.com/Masterminds/semver"
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

func SupportedVersion(protos []string) (map[string]*semver.Version, error) {
	if len(protos) == 0 {
		return nil, errors.New("empty input")
	}
	vs := make(map[string]*semver.Version)
	for _, el := range protos {
		a := strings.Split(el, "/")
		if len(a) < 1 {
			return nil, errors.New("fail to ")
		}
		major := a[len(a)-1]
		v, err := semver.NewVersion(major)
		if err != nil {
			return nil, fmt.Errorf("error parsing version: %w", err)
		}
		vs[el] = v
	}
	return vs, nil
}

func GetProtocol(ver string, supported map[string]*semver.Version) (protocol.ID, error) {
	a := strings.Split(ver, "gg")
	if len(a) < 2 {
		return "", errors.New("unsupported version format")
	}
	tv, err := semver.NewVersion(a[1])
	if err != nil {
		return "", err
	}
	for proto, v := range supported {
		if tv.Major() == v.Major() {
			return protocol.ConvertFromStrings([]string{proto})[0], nil
		}
	}
	return "", errors.New("no protocol matched with the given version")
}
