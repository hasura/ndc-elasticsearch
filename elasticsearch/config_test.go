package elasticsearch

import (
	"context"
	"errors"
	"testing"

	"github.com/elastic/go-elasticsearch/v8"
)

func TestSetupCredentailsUsingCredentialsProvider(t *testing.T) {
	t.Run("should return error if key is not set", func(t *testing.T) {
		esConfig := elasticsearch.Config{}
		err := setupCredentailsUsingCredentialsProvider(context.Background(), &esConfig, "", "", false)
		if !errors.Is(err, errCredentialProviderKeyNotSet) {
			t.Errorf("expected error to be errCredentialProviderKeyNotSet, got %v", err)
		}
	})

	t.Run("key is set", func(t *testing.T) {
		t.Run("should return error if mechanism is not set", func(t *testing.T) {
			esConfig := elasticsearch.Config{}
			err := setupCredentailsUsingCredentialsProvider(context.Background(), &esConfig, "key", "", false)
			if !errors.Is(err, errCredentialProviderMechanismNotSet) {
				t.Errorf("expected error to be errCredentialProviderMechanismNotSet, got %v", err)
			}
		})

		t.Run("should return error if mechanism is invalid", func(t *testing.T) {
			esConfig := elasticsearch.Config{}
			err := setupCredentailsUsingCredentialsProvider(context.Background(), &esConfig, "key", "invalid", false)
			if !errors.Is(err, errCredentialProviderMechanismInvalid) {
				t.Errorf("expected error to be errCredentialProviderMechanismInvalid, got %v", err)
			}
		})
	})
}
