package routerShared

import (
	"crypto/rand"
	"crypto/rsa"
	"errors"
)

var (
	ErrorInvalidTicket = errors.New("InvalidTicket")
)

type KeyRing struct {
	privateKey *rsa.PrivateKey
	publicKeys []*rsa.PublicKey
}

// creates new instance of `KeyRing`
func NewKeyRing() *KeyRing {
	privateKey, e := rsa.GenerateKey(rand.Reader, 4096)
	if e == nil {
		return &KeyRing{privateKey, []*rsa.PublicKey{&privateKey.PublicKey}}
	}
	panic("generate rsa private key error")
}

// returns own public key
func (ring *KeyRing) Public() ([]byte, error) {
	v, e := RSAPublicKeyEncode(ring.publicKeys[0])
	return v, e
}

// collects public key
func (ring *KeyRing) Add(v []byte) error {
	// TODO check for duplicates
	key, e := RSAPublicKeyDecode(v)
	if e == nil {
		ring.publicKeys = append(ring.publicKeys, key)
	}
	return e
}

// returns list of collected public keys
func (ring *KeyRing) Dump() (result [][]byte) {
	for _, key := range ring.publicKeys {
		v, e := RSAPublicKeyEncode(key)
		if e == nil {
			result = append(result, v)
		}
	}
	return result
}

// signs ticket with own private key
func (ring *KeyRing) JWTSign(claims *JWTClaims) (string, error) {
	return JWTSign(ring.privateKey, claims)
}

func (ring *KeyRing) JWTParse(ticket string) (*JWTClaims, error) {
	for _, key := range ring.publicKeys {
		claims, e := JWTParse(key, ticket)
		if e == nil {
			return claims, nil
		}
	}
	return nil, ErrorInvalidTicket
}
