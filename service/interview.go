package service

import (
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"log"
	"time"

	client "github.com/wamp3hub/wamp3go"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/xid"
)

type Interviewer struct {
	privateKey *rsa.PrivateKey
	session    *client.Session
}

func NewInterviewer(session *client.Session) (*Interviewer, error) {
	privateKey, e := rsa.GenerateKey(rand.Reader, 2048)
	if e == nil {
		instance := Interviewer{privateKey, session}
		return &instance, nil
	}
	return nil, e
}

func (interviewer *Interviewer) generatePeerID() string {
	return interviewer.session.ID() + "-" + xid.New().String()
}

func (interviewer *Interviewer) GenerateClaims(credentials any) (*jwt.RegisteredClaims, error) {
	log.Printf("[interviewer] credentials=%s", credentials)
	callEvent := client.NewCallEvent(&client.CallFeatures{"wamp.authenticate"}, credentials)
	replyEvent := interviewer.session.Call(callEvent)
	e := replyEvent.Error()
	if e != nil {
		if e.Error() != "ProcedureNotFound" {
			return nil, e
		}
		log.Printf("[interviewer] please, register `wamp.authenticate`")
	}

	now := time.Now()
	claims := jwt.RegisteredClaims{
		Issuer:    interviewer.session.ID(),
		Subject:   interviewer.generatePeerID(),
		ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute)),
		IssuedAt:  jwt.NewNumericDate(now),
	}
	return &claims, nil
}

func (interviewer *Interviewer) Encode(claims *jwt.RegisteredClaims) (string, error) {
	jwtoken := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token, e := jwtoken.SignedString(interviewer.privateKey)
	return token, e
}

func (interviewer *Interviewer) Decode(token string) (*jwt.RegisteredClaims, error) {
	jwtoken, e := jwt.ParseWithClaims(
		token,
		new(jwt.RegisteredClaims),
		func(token *jwt.Token) (any, error) {
			_, ok := token.Method.(*jwt.SigningMethodRSA)
			if !ok {
				return nil, errors.New("UnexpectedSigningMethod")
			}
			return interviewer.privateKey.Public(), nil
		},
	)

	if e == nil && jwtoken.Valid {
		claims, ok := jwtoken.Claims.(*jwt.RegisteredClaims)
		if ok {
			return claims, nil
		}
		e = errors.New("UnexpectedJWTClaims")
	}
	return nil, e
}
