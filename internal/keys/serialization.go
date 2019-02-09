package keys

import (
	"encoding/base32"

	"github.com/pkg/errors"
	"golang.org/x/crypto/ed25519"
)

func StringToKey(keyString string) (ed25519.PrivateKey, error) {
	b, err := base32.StdEncoding.DecodeString(keyString)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode key string")
	}

	return ed25519.PrivateKey(b), nil
}

func StringToPublicKey(keyString string) (ed25519.PublicKey, error) {
	b, err := base32.StdEncoding.DecodeString(keyString)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode key string")
	}

	return ed25519.PublicKey(b), nil
}

func KeyToString(key ed25519.PrivateKey) string {
	return base32.StdEncoding.EncodeToString(key)
}

func PublicKeyToString(key ed25519.PublicKey) string {
	return base32.StdEncoding.EncodeToString(key)
}
