package conversion

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"math/big"

	"github.com/btcsuite/btcd/btcec"
	"github.com/tendermint/tendermint/crypto/secp256k1"
	"golang.org/x/crypto/hkdf"
)

// UnmarshalSecp256k1PrivateKey returns a private key from bytes
func UnmarshalSecp256k1PrivateKey(data []byte) (*ecdsa.PrivateKey, error) {
	if len(data) != btcec.PrivKeyBytesLen {
		return nil, fmt.Errorf("expected secp256k1 data size to be %d", btcec.PrivKeyBytesLen)
	}

	sk, _ := btcec.PrivKeyFromBytes(btcec.S256(), data)
	return (*ecdsa.PrivateKey)(sk), nil
}

// UnmarshalSecp256k1PublicKey returns a public key from bytes
func UnmarshalSecp256k1PublicKey(data []byte) (*ecdsa.PublicKey, error) {
	k, err := btcec.ParsePubKey(data, btcec.S256())
	if err != nil {
		return nil, err
	}

	return (*ecdsa.PublicKey)(k), nil
}

func zeroPad(b []byte, length int) []byte {
	for i := 0; i < length-len(b); i++ {
		b = append([]byte{0x00}, b...)
	}
	return b
}

func kdf(secret []byte) (key []byte, err error) {
	key = make([]byte, 32)
	kdf := hkdf.New(sha256.New, secret, nil, nil)
	if _, err := io.ReadFull(kdf, key); err != nil {
		return nil, fmt.Errorf("fail to read the secret: %w", err)
	}
	return key, nil
}

func encapsulate(sk *ecdsa.PrivateKey, pk *ecdsa.PublicKey) ([]byte, error) {
	if pk == nil {
		return nil, fmt.Errorf("public key is empty")
	}

	var secret bytes.Buffer
	secret.Write(PkToBytes(&sk.PublicKey, false))
	sx, sy := pk.Curve.ScalarMult(pk.X, pk.Y, sk.D.Bytes())
	secret.Write([]byte{0x04})

	// padding if the public key coordinates less than 32 bytes
	l := len(pk.Curve.Params().P.Bytes())
	secret.Write(zeroPad(sx.Bytes(), l))
	secret.Write(zeroPad(sy.Bytes(), l))

	return kdf(secret.Bytes())
}

func decapsulate(pk *ecdsa.PublicKey, sk *ecdsa.PrivateKey) ([]byte, error) {
	if sk == nil {
		return nil, fmt.Errorf("public key is empty")
	}

	var secret bytes.Buffer

	secret.Write(PkToBytes(pk, false))
	sx, sy := sk.Curve.ScalarMult(pk.X, pk.Y, sk.D.Bytes())
	secret.Write([]byte{0x04})
	// padding if the public key coordinates less than 32 bytes
	l := len(sk.Curve.Params().P.Bytes())
	secret.Write(zeroPad(sx.Bytes(), l))
	secret.Write(zeroPad(sy.Bytes(), l))
	return kdf(secret.Bytes())
}

func PkToBytes(pk *ecdsa.PublicKey, compressed bool) []byte {
	x := pk.X.Bytes()
	if len(x) < 32 {
		for i := 0; i < 32-len(x); i++ {
			x = append([]byte{0}, x...)
		}
	}

	if compressed {
		// If odd
		if pk.Y.Bit(0) != 0 {
			return bytes.Join([][]byte{{0x03}, x}, nil)
		}

		// If even
		return bytes.Join([][]byte{{0x02}, x}, nil)
	}

	y := pk.Y.Bytes()
	if len(y) < 32 {
		for i := 0; i < 32-len(y); i++ {
			y = append([]byte{0}, y...)
		}
	}

	return bytes.Join([][]byte{{0x04}, x, y}, nil)
}

// Encrypt encrypts a passed message with a receiver public key, returns cipher text
func Encrypt(pubKey secp256k1.PubKeySecp256k1, msg []byte) ([]byte, error) {
	if len(pubKey) == 0 {
		return nil, errors.New("invalid pubkey")
	}
	var ct bytes.Buffer

	ephemeralSk := secp256k1.GenPrivKey()
	var ephemeralRaw [32]byte
	copy(ephemeralRaw[:], ephemeralSk[:])

	ecdsaPk, err := UnmarshalSecp256k1PublicKey(pubKey[:])
	if err != nil {
		return nil, err
	}

	ephemeralEcdsaSk, err := UnmarshalSecp256k1PrivateKey(ephemeralRaw[:])
	if err != nil {
		return nil, err
	}
	ct.Write(PkToBytes(&ephemeralEcdsaSk.PublicKey, false))

	// Derive shared secret
	ss, err := encapsulate(ephemeralEcdsaSk, ecdsaPk)
	if err != nil {
		return nil, err
	}

	// AES encryption
	block, err := aes.NewCipher(ss)
	if err != nil {
		return nil, fmt.Errorf("cannot create new aes block: %w", err)
	}

	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("cannot read random bytes for nonce: %w", err)
	}

	ct.Write(nonce)

	aesGcm, err := cipher.NewGCMWithNonceSize(block, 16)
	if err != nil {
		return nil, fmt.Errorf("cannot create aes gcm: %w", err)
	}

	cipherText := aesGcm.Seal(nil, nonce, msg, nil)

	tag := cipherText[len(cipherText)-aesGcm.NonceSize():]
	ct.Write(tag)
	cipherText = cipherText[:len(cipherText)-len(tag)]
	ct.Write(cipherText)

	return ct.Bytes(), nil
}

func Decrypt(sk secp256k1.PrivKeySecp256k1, msg []byte) ([]byte, error) {
	// Msg consist of 1+32+32+16(nonce)+tag(16)
	if len(sk) == 0 {
		return nil, errors.New("invalid private key")
	}
	if len(msg) <= (1 + 32 + 32 + 16 + 16) {
		return nil, fmt.Errorf("invalid length of message")
	}
	// Ephemeral sender public key
	ephemeralPubkey := &ecdsa.PublicKey{
		Curve: btcec.S256(),
		X:     new(big.Int).SetBytes(msg[1:33]),
		Y:     new(big.Int).SetBytes(msg[33:65]),
	}

	var skRaw [32]byte
	copy(skRaw[:], sk[:])
	ecdsaSk, err := UnmarshalSecp256k1PrivateKey(skRaw[:])
	if err != nil {
		return nil, err
	}
	// Derive shared secret
	ss, err := decapsulate(ephemeralPubkey, ecdsaSk)
	if err != nil {
		return nil, err
	}
	// msg without pk
	msg = msg[65:]
	// AES decryption part
	nonce := msg[:16]
	tag := msg[16:32]

	// Create Golang-accepted ciphertext
	ciphertext := bytes.Join([][]byte{msg[32:], tag}, nil)

	block, err := aes.NewCipher(ss)
	if err != nil {
		return nil, fmt.Errorf("cannot create new aes block: %w", err)
	}

	gcm, err := cipher.NewGCMWithNonceSize(block, 16)
	if err != nil {
		return nil, fmt.Errorf("cannot create gcm cipher: %w", err)
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot decrypt ciphertext: %w", err)
	}

	return plaintext, nil
}
