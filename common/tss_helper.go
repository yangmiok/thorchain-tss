package common

import (
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"os"
	"sort"
	"strconv"

	btss "github.com/binance-chain/tss-lib/tss"
	sdk "github.com/cosmos/cosmos-sdk/types"
	mapset "github.com/deckarep/golang-set"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/tendermint/btcd/btcec"
	"github.com/tendermint/tendermint/crypto/secp256k1"

	"gitlab.com/thorchain/tss/go-tss/messages"
)

func Contains(s []*btss.PartyID, e *btss.PartyID) bool {
	if e == nil {
		return false
	}
	for _, a := range s {
		if *a == *e {
			return true
		}
	}
	return false
}

func GetThreshold(value int) (int, error) {
	if value < 0 {
		return 0, errors.New("negative input")
	}
	threshold := int(math.Ceil(float64(value)*2.0/3.0)) - 1
	return threshold, nil
}

func SetupPartyIDMap(partiesID []*btss.PartyID) map[string]*btss.PartyID {
	partyIDMap := make(map[string]*btss.PartyID)
	for _, id := range partiesID {
		partyIDMap[id.Id] = id
	}
	return partyIDMap
}

func GetPeersID(partyIDtoP2PID map[string]peer.ID, localPeerID string) []peer.ID {
	peerIDs := make([]peer.ID, 0, len(partyIDtoP2PID)-1)
	for _, value := range partyIDtoP2PID {
		if value.String() == localPeerID {
			continue
		}
		peerIDs = append(peerIDs, value)
	}
	return peerIDs
}

func SetupIDMaps(parties map[string]*btss.PartyID, partyIDtoP2PID map[string]peer.ID) error {
	for id, party := range parties {
		peerID, err := getPeerIDFromPartyID(party)
		if err != nil {
			return err
		}
		partyIDtoP2PID[id] = peerID
	}
	return nil
}

func MsgToHashInt(msg []byte) (*big.Int, error) {
	h := sha256.New()
	_, err := h.Write(msg)
	if err != nil {
		return nil, fmt.Errorf("fail to caculate sha256 hash: %w", err)
	}
	return hashToInt(h.Sum(nil), btcec.S256()), nil
}

func MsgToHashString(msg []byte) (string, error) {
	h := sha256.New()
	_, err := h.Write(msg)
	if err != nil {
		return "", fmt.Errorf("fail to caculate sha256 hash: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func hashToInt(hash []byte, c elliptic.Curve) *big.Int {
	orderBits := c.Params().N.BitLen()
	orderBytes := (orderBits + 7) / 8
	if len(hash) > orderBytes {
		hash = hash[:orderBytes]
	}

	ret := new(big.Int).SetBytes(hash)
	excess := len(hash)*8 - orderBits
	if excess > 0 {
		ret.Rsh(ret, uint(excess))
	}
	return ret
}

func InitLog(level string, pretty bool, serviceValue string) {
	l, err := zerolog.ParseLevel(level)
	if err != nil {
		log.Warn().Msgf("%s is not a valid log-level, falling back to 'info'", level)
	}
	var out io.Writer = os.Stdout
	if pretty {
		out = zerolog.ConsoleWriter{Out: os.Stdout}
	}
	zerolog.SetGlobalLevel(l)
	log.Logger = log.Output(out).With().Str("service", serviceValue).Logger()
}

func partyIDtoPubKey(party *btss.PartyID) (string, error) {
	partyKeyBytes := party.GetKey()
	var pk secp256k1.PubKeySecp256k1
	copy(pk[:], partyKeyBytes)
	pubKey, err := sdk.Bech32ifyAccPub(pk)
	if err != nil {
		return "", err
	}
	return pubKey, nil
}

func AccPubKeysFromPartyIDs(partyIDs []string, partyIDMap map[string]*btss.PartyID) ([]string, error) {
	pubKeys := make([]string, 0)
	for _, partyID := range partyIDs {
		blameParty, ok := partyIDMap[partyID]
		if !ok {
			return nil, errors.New("cannot find the blame party")
		}
		blamePubKey, err := partyIDtoPubKey(blameParty)
		if err != nil {
			return nil, err
		}
		pubKeys = append(pubKeys, blamePubKey)
	}
	return pubKeys, nil
}

// GetBlamePubKeysInList returns the nodes public key who are in the peer list
func (t *TssCommon) getBlamePubKeysInList(peers []string) ([]string, error) {
	var partiesInList []string
	// we convert nodes (in the peers list) P2PID to public key
	for partyID, p2pID := range t.PartyIDtoP2PID {
		for _, el := range peers {
			if el == p2pID.String() {
				partiesInList = append(partiesInList, partyID)
			}
		}
	}

	localPartyInfo := t.getPartyInfo()
	partyIDMap := localPartyInfo.PartyIDMap
	blamePubKeys, err := AccPubKeysFromPartyIDs(partiesInList, partyIDMap)
	if err != nil {
		return nil, err
	}

	return blamePubKeys, nil
}

func (t *TssCommon) getBlamePubKeysNotInList(peers []string) ([]string, error) {
	var partiesNotInList []string
	// we convert nodes (NOT in the peers list) P2PID to public key
	for partyID, p2pID := range t.PartyIDtoP2PID {
		if t.partyInfo.Party.PartyID().Id == partyID {
			continue
		}
		found := false
		for _, each := range peers {
			if p2pID.String() == each {
				found = true
				break
			}
		}
		if found == false {
			partiesNotInList = append(partiesNotInList, partyID)
		}
	}

	localPartyInfo := t.getPartyInfo()
	partyIDMap := localPartyInfo.PartyIDMap
	blamePubKeys, err := AccPubKeysFromPartyIDs(partiesNotInList, partyIDMap)
	if err != nil {
		return nil, err
	}

	return blamePubKeys, nil
}

// GetBlamePubKeysNotInList returns the nodes public key who are not in the peer list
func (t *TssCommon) GetBlamePubKeysLists(peer []string) ([]string, []string, error) {
	inList, err := t.getBlamePubKeysInList(peer)
	if err != nil {
		return nil, nil, err
	}

	notInlist, err := t.getBlamePubKeysNotInList(peer)
	if err != nil {
		return nil, nil, err
	}

	return inList, notInlist, err
}

// this blame blames the node who provide the wrong share
func (t *TssCommon) TssWrongShareBlame(wiredMsg *messages.WireMessage) string {
	shareOwner := wiredMsg.Routing.From
	owner, ok := t.getPartyInfo().PartyIDMap[shareOwner.Id]
	if !ok {
		t.logger.Error().Msg("cannot find the blame node public key")
		return ""
	}
	pk, err := partyIDtoPubKey(owner)
	if err != nil {
		return ""
	}
	return pk
}

// for the last round, we first check whether the stored share which means it get the majority of the peers to accept
// this share. We then, find the missing one to blame, if he is honest he must get 2/3 peers accept his share(given the
// condition that 2/3 of the peers are honest)
func (t *TssCommon) TssTimeoutBlame(lastMessageType string) ([]string, error) {

	peersSet := mapset.NewSet()
	standbySet := mapset.NewSet()
	for _, el := range t.partyInfo.PartyIDMap {
		if el.Id != t.partyInfo.Party.PartyID().Id {
			peersSet.Add(el.Id)
		}
	}

	for _, el := range t.msgStored.storedMsg {
		if el.RoundInfo == lastMessageType {
			standbySet.Add(el.Routing.From.Id)
		}
	}

	var blames []string
	diff := peersSet.Difference(standbySet).ToSlice()
	for _, el := range diff {
		blames = append(blames, el.(string))
	}

	blamePubKeys, err := AccPubKeysFromPartyIDs(blames, t.partyInfo.PartyIDMap)
	if err != nil {
		t.logger.Error().Err(err).Msg("fail to get the public keys of the blame node")
		return nil, err
	}

	return blamePubKeys, nil
}

func getHighestFreq(confirmedList map[string]string) (string, int, error) {
	if len(confirmedList) == 0 {
		return "", 0, errors.New("empty input")
	}
	freq := make(map[string]int, len(confirmedList))
	hashPeerMap := make(map[string]string, len(confirmedList))
	for peer, n := range confirmedList {
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

func (t *TssCommon) GetUnicastBlame(msgType string) ([]BlameNode, error) {
	peersID, ok := t.lastUnicastPeer[msgType]
	if !ok {
		t.logger.Error().Msg("fail to get the blamed peers")
		return nil, fmt.Errorf("fail to get the blamed peers %w", ErrTssTimeOut)
	}
	// use map to rule out the peer duplication
	peersMap := make(map[string]bool)
	for _, el := range peersID {
		peersMap[el.String()] = true
	}
	var onlinePeers []string
	for key, _ := range peersMap {
		onlinePeers = append(onlinePeers, key)
	}
	_, blamePeers, err := t.GetBlamePubKeysLists(onlinePeers)
	if err != nil {
		t.logger.Error().Err(err).Msg("fail to get the blamed peers")
		return nil, fmt.Errorf("fail to get the blamed peers %w", ErrTssTimeOut)
	}
	var blameNodes []BlameNode
	for _, el := range blamePeers {
		bn := BlameNode{
			Pubkey:         el,
			BlameData:      nil,
			BlameSignature: nil,
		}
		blameNodes = append(blameNodes, bn)
	}
	return blameNodes, nil
}

func (t *TssCommon) GetBroadcastBlame(lastMessageType string) ([]BlameNode, error) {

	blamePeers, err := t.TssTimeoutBlame(lastMessageType)
	if err != nil {
		t.logger.Error().Err(err).Msg("fail to get the blamed peers")
		return nil, fmt.Errorf("fail to get the blamed peers %w", ErrTssTimeOut)
	}
	var blameNodes []BlameNode
	for _, el := range blamePeers {
		bn := NewBlameNode(el, nil, nil)
		blameNodes = append(blameNodes, bn)
	}
	return blameNodes, nil
}
