package service

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"log"

	wamp "github.com/wamp3hub/wamp3go"
	router "github.com/wamp3hub/wamp3router"

	"github.com/golang-jwt/jwt/v5"
)

func PublicKeyEncode(v *rsa.PublicKey) ([]byte, error) {
	vbytes, e := x509.MarshalPKIXPublicKey(v)
	if e == nil {
		vpem := pem.EncodeToMemory(&pem.Block{Type: "RSA PUBLIC KEY", Bytes: vbytes})
		return vpem, nil
	}
	return vbytes, e
}

func PublicKeyDecode(v []byte) (*rsa.PublicKey, error) {
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

type Claims = jwt.RegisteredClaims

func JWTEncode(key *rsa.PrivateKey, claims *Claims) (string, error) {
	jwtoken := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	ticket, e := jwtoken.SignedString(key)
	return ticket, e
}

func JWTDecode(key *rsa.PublicKey, ticket string) (*Claims, error) {
	jwtoken, e := jwt.ParseWithClaims(
		ticket,
		new(Claims),
		func(token *jwt.Token) (any, error) {
			_, ok := token.Method.(*jwt.SigningMethodRSA)
			if ok {
				return key, nil
			}
			return nil, errors.New("UnexpectedSigningMethod")
		},
	)

	if e == nil {
		claims, ok := jwtoken.Claims.(*Claims)
		if ok {
			return claims, nil
		}
		e = errors.New("UnexpectedJWTClaims")
	}
	return nil, e
}

type KeyRing struct {
	privateKey *rsa.PrivateKey
	// TODO must be set
	publicKeys []*rsa.PublicKey
}

func NewKeyRing() *KeyRing {
	privateKey, e := rsa.GenerateKey(rand.Reader, 2048)
	if e == nil {
		return &KeyRing{
			privateKey,
			[]*rsa.PublicKey{&privateKey.PublicKey},
		}
	}
	panic("generate rsa private key error")
}

func (ring *KeyRing) Public() (string, error) {
	v, e := PublicKeyEncode(ring.publicKeys[0])
	return string(v), e
}

func (ring *KeyRing) Add(v string) error {
	key, e := PublicKeyDecode([]byte(v))
	if e == nil {
		ring.publicKeys = append(ring.publicKeys, key)
	}
	return e
}

func (ring *KeyRing) Dump() (result []string) {
	for _, key := range ring.publicKeys {
		v, e := PublicKeyEncode(key)
		if e == nil {
			result = append(result, string(v))
		}
	}
	return result
}

func (ring *KeyRing) JWTEncode(claims *Claims) (string, error) {
	return JWTEncode(ring.privateKey, claims)
}

func (ring *KeyRing) JWTDecode(ticket string) (*Claims, error) {
	for _, key := range ring.publicKeys {
		claims, e := JWTDecode(key, ticket)
		if e == nil {
			return claims, nil
		}
	}
	return nil, errors.New("InvalidTicket")
}

func ShareClusterKeys(ring *KeyRing, session *wamp.Session) error {
	_, e := session.Subscribe(
		"wamp.cluster.key.public.new",
		&wamp.SubscribeOptions{},
		func(publishEvent wamp.PublishEvent) {
			var key string
			e := publishEvent.Payload(&key)
			if e == nil {
				ring.Add(key)
			}
		},
	)
	if e != nil {
		log.Printf("wamp.cluster.key.public.new subscribe error %s", e)
		return e
	}

	_, e = session.Register(
		"wamp.cluster.key.public.list",
		&wamp.RegisterOptions{},
		func(callEvent wamp.CallEvent) wamp.ReplyEvent {
			keyList := ring.Dump()
			return wamp.NewReplyEvent(callEvent, keyList)
		},
	)
	if e != nil {
		log.Printf("wamp.cluster.key.public.list register error %s", e)
		return e
	}

	return nil
}

func LoadClusterKeys(ring *KeyRing, cluster *EventDistributor) error {
	callEvent := wamp.NewCallEvent(&wamp.CallFeatures{"wamp.cluster.key.public.list"}, router.Emptiness{})
	replyEvent := cluster.Call(callEvent)
	keyList := []string{}
	e := replyEvent.Payload(&keyList)
	if e == nil {
		for _, key := range keyList {
			ring.Add(key)
		}

		key, e := ring.Public()
		if e == nil {
			publishEvent := wamp.NewPublishEvent(&wamp.PublishFeatures{URI: "wamp.cluster.key.public.new"}, key)
			cluster.Publish(publishEvent)
		}
	}

	return e
}
