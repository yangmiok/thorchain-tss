package p2p

import (
	"context"
	"fmt"
	"sync"

	"github.com/libp2p/go-libp2p-core/peer"
	dht "github.com/libp2p/go-libp2p-kad-dht"
)

type CachedP2PAddr struct {
	cachedP2PAddr map[peer.ID]peer.AddrInfo
	kadDHT        *dht.IpfsDHT
	lock          *sync.RWMutex
}

func NewCachedP2PAddr() *CachedP2PAddr {
	return &CachedP2PAddr{
		cachedP2PAddr: make(map[peer.ID]peer.AddrInfo),
		lock:          &sync.RWMutex{},
	}
}

func (addrCache *CachedP2PAddr) UpdateDHT(kadDHT *dht.IpfsDHT) {
	addrCache.kadDHT = kadDHT
}

func (addrCache *CachedP2PAddr) Get(peer peer.ID) (peer.AddrInfo, bool) {
	addrCache.lock.RLock()
	defer addrCache.lock.RUnlock()
	val, ok := addrCache.cachedP2PAddr[peer]
	return val, ok
}

func (addrCache *CachedP2PAddr) Put(pid peer.ID, addr peer.AddrInfo) {
	addrCache.lock.Lock()
	defer addrCache.lock.Unlock()
	addrCache.cachedP2PAddr[pid] = addr
}

func (addrCache *CachedP2PAddr) BulkDelete(peers []peer.ID) {
	addrCache.lock.Lock()
	defer addrCache.lock.Unlock()
	for _, peer := range peers {
		delete(addrCache.cachedP2PAddr, peer)
	}
}

func (addrCache *CachedP2PAddr) FindPeer(ctx context.Context, pid peer.ID) (peer.AddrInfo, error) {
	target, ok := addrCache.Get(pid)
	fmt.Println("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if ok {
		return target, nil
	}
	fmt.Printf("WWWWWWWWWWWWWWWWWWWWWWWWWWWWWWWWWWWWWWWW")
	target, err := addrCache.kadDHT.FindPeer(ctx, pid)
	if err != nil {
		return peer.AddrInfo{}, err
	}
	addrCache.Put(pid, target)
	return target, nil
}
