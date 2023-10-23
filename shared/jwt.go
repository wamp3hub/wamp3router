package shared

import (
	"crypto/rsa"
	"errors"

	"github.com/golang-jwt/jwt/v5"
)

type JWTClaims = jwt.RegisteredClaims

func JWTSign(key *rsa.PrivateKey, claims *JWTClaims) (string, error) {
	jwtoken := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	ticket, e := jwtoken.SignedString(key)
	return ticket, e
}

func JWTParse(key *rsa.PublicKey, ticket string) (*JWTClaims, error) {
	jwtoken, e := jwt.ParseWithClaims(
		ticket,
		new(JWTClaims),
		func(token *jwt.Token) (any, error) {
			_, ok := token.Method.(*jwt.SigningMethodRSA)
			if ok {
				return key, nil
			}
			return nil, errors.New("UnexpectedSigningMethod")
		},
	)

	if e == nil {
		claims, ok := jwtoken.Claims.(*JWTClaims)
		if ok {
			return claims, nil
		}
		e = errors.New("UnexpectedJWTClaims")
	}
	return nil, e
}
