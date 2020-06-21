module gitlab.com/thorchain/tss/go-tss

go 1.13

require (
	github.com/binance-chain/go-sdk v1.2.2
	github.com/binance-chain/tss-lib v1.3.1
	github.com/btcsuite/btcd v0.20.1-beta.0.20200414114020-8b54b0b96418
	github.com/cosmos/cosmos-sdk v0.38.4
	github.com/deckarep/golang-set v1.7.1
	github.com/decred/dcrd/dcrec/secp256k1 v1.0.3
	github.com/gogo/protobuf v1.3.1
	github.com/golang/protobuf v1.3.4
	github.com/gorilla/mux v1.7.4
	github.com/ipfs/go-log v1.0.2
	github.com/libp2p/go-libp2p v0.5.2
	github.com/libp2p/go-libp2p-core v0.3.1
	github.com/libp2p/go-libp2p-discovery v0.2.0
	github.com/libp2p/go-libp2p-kad-dht v0.5.0
	github.com/libp2p/go-libp2p-peerstore v0.1.4
	github.com/libp2p/go-libp2p-testing v0.1.1
	github.com/multiformats/go-multiaddr v0.2.1
	github.com/rs/zerolog v1.18.0
	github.com/stretchr/testify v1.5.1
	github.com/tendermint/btcd v0.1.1
	github.com/tendermint/tendermint v0.33.3
	gitlab.com/thorchain/thornode v0.0.0-20200619001200-6ecf07ecf5f3
	golang.org/x/crypto v0.0.0-20200510223506-06a226fb4e37
	gopkg.in/check.v1 v1.0.0-20200227125254-8fa46927fb4f
)

replace github.com/binance-chain/go-sdk => gitlab.com/thorchain/binance-sdk v1.2.2
