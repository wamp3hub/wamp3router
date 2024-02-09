package routerShared

import (
	"crypto/rsa"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	wampInterview "github.com/wamp3hub/wamp3go/interview"
)

var (
	ErrorUnexpectedJWTClaims = errors.New("UnexpectedJWTClaims")
)

type JWTClaims struct {
	jwt.RegisteredClaims
	Role  string              `json:"role"`
	Offer wampInterview.Offer `json:"offer"`
}

func NewJWTClaims(
	issuer string,
	peer string,
	role string,
	offer *wampInterview.Offer,
	expires time.Duration,
) *JWTClaims {
	now := time.Now()
	return &JWTClaims{
		Role:  role,
		Offer: *offer,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   peer,
			ExpiresAt: jwt.NewNumericDate(now.Add(expires)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
}

func JWTSign(key *rsa.PrivateKey, claims *JWTClaims) (string, error) {
	jwtoken := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	ticket, e := jwtoken.SignedString(key)
	return ticket, e
}

func jwtJustParse(ticket string) (*JWTClaims, error) {
	jwtParser := jwt.NewParser()
	jwtoken, _, e := jwtParser.ParseUnverified(ticket, new(JWTClaims))
	if e == nil {
		claims, ok := jwtoken.Claims.(*JWTClaims)
		if ok {
			return claims, nil
		}
		e = ErrorUnexpectedJWTClaims
	}
	return nil, e
}

func jwtVerifyParse(key *rsa.PublicKey, ticket string) (*JWTClaims, error) {
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
		e = ErrorUnexpectedJWTClaims
	}
	return nil, e
}

func JWTParse(key *rsa.PublicKey, ticket string) (*JWTClaims, error) {
	if key == nil {
		return jwtJustParse(ticket)
	}
	return jwtVerifyParse(key, ticket)
}
