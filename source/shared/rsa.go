package routerShared

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
)

var (
	ErrorInvalidRSA = errors.New("invalid RSA key")
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
		return nil, ErrorInvalidRSA
	}

	publicKey, e := x509.ParsePKIXPublicKey(block.Bytes)
	if e == nil {
		publicKey, ok := publicKey.(*rsa.PublicKey)
		if ok {
			return publicKey, nil
		}
		e = ErrorInvalidRSA
	}
	return nil, e
}

func ReadRSAPublicKey(path string) (key *rsa.PublicKey, e error) {
	bytes, e := os.ReadFile(path)
	if e == nil {
		key, e = RSAPublicKeyDecode(bytes)
	}
	return key, e
}

func WriteRSAPublicKey(path string, key *rsa.PublicKey) error {
	file, e := os.Create(path)
	if e == nil {
		bytes, _ := RSAPublicKeyEncode(key)
		_, e = file.Write(bytes)
	}
	return e
}

func RSAPrivateKeyEncode(v *rsa.PrivateKey) ([]byte, error) {
	vbytes, e := x509.MarshalPKCS8PrivateKey(v)
	if e == nil {
		vpem := pem.EncodeToMemory(&pem.Block{Type: "RSA PUBLIC KEY", Bytes: vbytes})
		return vpem, nil
	}
	return vbytes, e
}

func RSAPrivateKeyDecode(v []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(v)
	if block == nil {
		return nil, ErrorInvalidRSA
	}

	privateKey, e := x509.ParsePKCS8PrivateKey(block.Bytes)
	if e == nil {
		privateKey, ok := privateKey.(*rsa.PrivateKey)
		if ok {
			return privateKey, nil
		}
		e = ErrorInvalidRSA
	}
	return nil, e
}

func ReadRSAPrivateKey(path string) (key *rsa.PrivateKey, e error) {
	bytes, e := os.ReadFile(path)
	if e == nil {
		key, e = RSAPrivateKeyDecode(bytes)
	}
	return key, e
}

func WriteRSAPrivateKey(path string, key *rsa.PrivateKey) error {
	file, e := os.Create(path)
	if e == nil {
		bytes, _ := RSAPrivateKeyEncode(key)
		_, e = file.Write(bytes)
	}
	return e
}
