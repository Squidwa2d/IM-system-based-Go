package token

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrExpiredToken = errors.New("token has expired")
	ErrInvalidToken = errors.New("invalid token")
)

// Payload contains the data of the token
type Payload struct {
	ID        pgtype.UUID        `json:"id"`
	Username  string             `json:"username"`
	IssuedAt  pgtype.Timestamptz `json:"issued_at"`
	ExpiredAt pgtype.Timestamptz `json:"expired_at"`
}

func NewPayload(username string, duration time.Duration) (*Payload, error) {
	tokenID, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}

	payload := &Payload{
		ID: pgtype.UUID{
			Bytes: tokenID,
			Valid: true,
		},
		Username: username,
		IssuedAt: pgtype.Timestamptz{
			Time:  time.Now(),
			Valid: true,
		},
		ExpiredAt: pgtype.Timestamptz{
			Time:  time.Now().Add(duration),
			Valid: true,
		},
	}

	return payload, nil
}

func (payload *Payload) Valid() error {
	if time.Now().After(payload.ExpiredAt.Time) {
		return ErrExpiredToken
	}
	return nil
}
