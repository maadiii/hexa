package infra

import (
	"golang.org/x/crypto/bcrypt"
	"hexa/internal/core/port"
)

type Hasher struct{}

func NewHasher() port.Hasher {
	return &Hasher{}
}

func (b *Hasher) Hash(password string) (string, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	return string(hashedPassword), nil
}

func (b *Hasher) Compare(hashedPassword, textPassword string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(textPassword))
}
