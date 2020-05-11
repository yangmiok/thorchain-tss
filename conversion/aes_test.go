package conversion

import (
	"github.com/tendermint/tendermint/crypto/secp256k1"
	. "gopkg.in/check.v1"
)

const testingMessage = "helloworldAES"

type AesTestSuite struct{}

var _ = Suite(&AesTestSuite{})

func (aes *AesTestSuite) TestEncryptAndDecrypt(c *C) {
	testSk := secp256k1.GenPrivKey()
	var pk secp256k1.PubKeySecp256k1
	copy(pk[:], testSk.PubKey().Bytes())

	testPk := testSk.PubKey().(secp256k1.PubKeySecp256k1)

	cipher, err := Encrypt(testPk, []byte(testingMessage))
	c.Assert(err, IsNil)
	plaintext, err := Decrypt(testSk, cipher)
	c.Assert(err, IsNil)
	c.Assert(testingMessage, Equals, string(plaintext))
}

func (aes *AesTestSuite) TestEncryptionFailed(c *C) {
	a := secp256k1.PubKeySecp256k1{}
	_, err := Encrypt(a, []byte(testingMessage))
	c.Assert(err, NotNil)
	testSk := secp256k1.GenPrivKey()
	testSk2 := secp256k1.GenPrivKey()
	var pk secp256k1.PubKeySecp256k1
	copy(pk[:], testSk.PubKey().Bytes())
	testPk := testSk.PubKey().(secp256k1.PubKeySecp256k1)

	cipher, err := Encrypt(testPk, []byte(testingMessage))
	c.Assert(err, IsNil)
	_, err = Decrypt(testSk2, cipher)
	c.Assert(err, ErrorMatches, "cannot decrypt ciphertext: cipher: message authentication failed")
}
