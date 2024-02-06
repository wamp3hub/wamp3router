package routerShared_test

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	wampShared "github.com/wamp3hub/wamp3go/shared"
	routerShared "github.com/wamp3hub/wamp3router/source/shared"
)

func TestKeyRing(t *testing.T) {
	keyRing := routerShared.GenerateKeyRing()

	myPublicKey, e := keyRing.Public()
	if e != nil {
		t.Fatalf("Invalid behaviour %s", e)
	}

	e = keyRing.Add(myPublicKey)
	if e != nil {
		t.Fatalf("Invalid behaviour %s", e)
	}

	publicKeys := keyRing.Dump()
	if len(publicKeys) != 2 {
		t.Fatalf("Invalid behaviour")
	}

	now := time.Now()
	expectedClaims := routerShared.JWTClaims{
		Issuer:    wampShared.NewID(),
		Subject:   wampShared.NewID(),
		ExpiresAt: jwt.NewNumericDate(now.Add(7 * 24 * time.Hour)),
		IssuedAt:  jwt.NewNumericDate(now),
	}
	ticket, e := keyRing.JWTSign(&expectedClaims)
	if e != nil {
		t.Fatalf("Invalid behaviour %s", e)
	}

	claims, e := keyRing.JWTParse(ticket)
	if e != nil {
		t.Fatalf("Invalid behaviour %s", e)
	}

	if claims.Issuer != expectedClaims.Issuer {
		t.Fatalf("JWTParse expected %v, but got %v", expectedClaims.Issuer, claims.Issuer)
	}

	_, e = keyRing.JWTParse("invalid-ticket")
	if e == nil {
		t.Fatalf("Invalid behaviour")
	}

	claims, e = routerShared.JWTParse(nil, ticket)
	if e != nil {
		t.Fatalf("Invalid behaviour %s", e)
	}

	if claims.Subject != expectedClaims.Subject {
		t.Fatalf("JWTParse expected %v, but got %v", expectedClaims.Subject, claims.Subject)
	}
}
