package routerShared

import (
	"crypto/rand"
	"crypto/rsa"
	"errors"
)

var (
	ErrorInvalidTicket = errors.New("invalid ticket")
)

type KeyRing struct {
	PrivateKey *rsa.PrivateKey
	PublicKeys []*rsa.PublicKey
}

// creates new instance of `KeyRing`
func NewKeyRing(
	privateKey *rsa.PrivateKey,
) *KeyRing {
	return &KeyRing{privateKey, []*rsa.PublicKey{&privateKey.PublicKey}}
}

// creates new instance of `KeyRing`
func GenerateKeyRing() *KeyRing {
	privateKey, e := rsa.GenerateKey(rand.Reader, 4096)
	if e != nil {
		panic("generate rsa private key error")
	}
	return NewKeyRing(privateKey)
}

// returns own public key
func (ring *KeyRing) Public() ([]byte, error) {
	v, e := RSAPublicKeyEncode(ring.PublicKeys[0])
	return v, e
}

// collects public key
func (ring *KeyRing) Add(v []byte) error {
	// TODO check for duplicates
	key, e := RSAPublicKeyDecode(v)
	if e == nil {
		ring.PublicKeys = append(ring.PublicKeys, key)
	}
	return e
}

// returns list of collected public keys
func (ring *KeyRing) Dump() (result [][]byte) {
	for _, key := range ring.PublicKeys {
		v, e := RSAPublicKeyEncode(key)
		if e == nil {
			result = append(result, v)
		}
	}
	return result
}

// signs ticket with own private key
func (ring *KeyRing) JWTSign(claims *JWTClaims) (string, error) {
	return JWTSign(ring.PrivateKey, claims)
}

func (ring *KeyRing) JWTParse(ticket string) (*JWTClaims, error) {
	for _, key := range ring.PublicKeys {
		claims, e := JWTParse(key, ticket)
		if e == nil {
			return claims, nil
		}
	}
	return nil, ErrorInvalidTicket
}
