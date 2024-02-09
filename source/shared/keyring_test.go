package routerShared_test

import (
	"testing"

	"github.com/golang-jwt/jwt/v5"
	wampInterview "github.com/wamp3hub/wamp3go/interview"
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

	expectedRole := "guest"
	expectedOffer := wampInterview.Offer{
		RegistrationsLimit: 1,
		SubscriptionsLimit: 1,
	}
	expectedClaims := routerShared.JWTClaims{
		jwt.RegisteredClaims{
			Issuer: wampShared.NewID(),
		},
		expectedRole,
		expectedOffer,
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

	if claims.Role != expectedRole {
		t.Fatalf("JWTParse expected %v, but got %v", claims.Role, expectedRole)
	}

	if claims.Offer.RegistrationsLimit != expectedOffer.RegistrationsLimit ||
		claims.Offer.SubscriptionsLimit != expectedOffer.SubscriptionsLimit {
		t.Fatalf("JWT Offer parse error")
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
