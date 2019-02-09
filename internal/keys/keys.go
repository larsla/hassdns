package keys

import (
	"crypto/rand"

	"github.com/pkg/errors"
	"golang.org/x/crypto/ed25519"
)

func Generate() (ed25519.PrivateKey, error) {
	_, pri, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate key")
	}

	return pri, nil
}
