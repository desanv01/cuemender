package twitch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/desanv01/cuemender/internal/secrets"
)

// CredentialStore serializes the access/refresh pair and binding metadata as
// one application-owned secret entry. It never falls back to plaintext.
type CredentialStore struct {
	store secrets.Store
	key   string
}

func NewCredentialStore(store secrets.Store) *CredentialStore {
	return &CredentialStore{store: store, key: credentialKey}
}

func (s *CredentialStore) Load(ctx context.Context) (Credentials, error) {
	raw, err := s.store.Get(ctx, s.key)
	if err != nil {
		if secrets.IsNotFound(err) {
			return Credentials{}, ErrNoCredentials
		}
		return Credentials{}, ErrCredentialsUnavailable
	}
	var creds Credentials
	if err := json.Unmarshal([]byte(raw), &creds); err != nil {
		return Credentials{}, fmt.Errorf("decode stored twitch credentials")
	}
	if creds.AccessToken == "" || creds.RefreshToken == "" || creds.ClientID == "" || creds.UserID == "" {
		return Credentials{}, errors.New("incomplete stored twitch credentials")
	}
	return creds, nil
}

func (s *CredentialStore) Save(ctx context.Context, creds Credentials) error {
	raw, err := json.Marshal(creds)
	if err != nil {
		return fmt.Errorf("encode twitch credentials")
	}
	if err := s.store.Set(ctx, s.key, string(raw)); err != nil {
		if secrets.IsUnavailable(err) {
			return ErrCredentialsUnavailable
		}
		return errors.New("save twitch credentials")
	}
	return nil
}

func (s *CredentialStore) Delete(ctx context.Context) error {
	err := s.store.Delete(ctx, s.key)
	if err == nil || secrets.IsNotFound(err) {
		return nil
	}
	if secrets.IsUnavailable(err) {
		return ErrCredentialsUnavailable
	}
	return errors.New("delete twitch credentials")
}
