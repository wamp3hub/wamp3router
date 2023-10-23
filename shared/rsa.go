package shared

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
)

func RSAPublicKeyEncode(v *rsa.PublicKey) ([]byte, error) {
	vbytes, e := x509.MarshalPKIXPublicKey(v)
	if e == nil {
		vpem := pem.EncodeToMemory(&pem.Block{Type: "RSA PUBLIC KEY", Bytes: vbytes})
		return vpem, nil
	}
	return vbytes, e
}

func RSAPublicKeyDecode(v []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(v)
	if block == nil {
		return nil, errors.New("parse error (PEM block containing the key)")
	}

	publicKey, e := x509.ParsePKIXPublicKey(block.Bytes)
	if e == nil {
		publicKey, ok := publicKey.(*rsa.PublicKey)
		if ok {
			return publicKey, nil
		}
		e = errors.New("key type is not RSA")
	}
	return nil, e
}
