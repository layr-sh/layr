package filestorage

import (
	"context"
	"strings"
	"testing"
	"uuid"

	"layr.sh/core"
)

func createTestServiceAccountWithS3Credentials(
	t *testing.T,
	kernel *core.Kernel,
	name string,
	scopes []string,
) (uuid.UUID, string, string) {
	t.Helper()
	ctx := context.Background()

	serviceAccount, err := kernel.ServiceAccountManager().Create(ctx, core.CreateServiceAccountInput{
		Name:   name,
		Scopes: scopes,
	})
	if err != nil {
		t.Fatalf("failed to create service account: %v", err)
	}

	parsedID, err := uuid.Parse(serviceAccount.ID)
	if err != nil {
		t.Fatalf("failed to parse service account id: %v", err)
	}

	accessKeyID := "LFS" + strings.ToUpper(strings.ReplaceAll(uuid.NewV7().String(), "-", ""))[:17]
	secretAccessKey := serviceAccount.SecretKey

	encryptedSecretKey, err := kernel.CryptoKeyManager().EncryptField([]byte(secretAccessKey))
	if err != nil {
		t.Fatalf("failed to encrypt s3 secret key: %v", err)
	}

	const insertCredentialSQL = `
		INSERT INTO file_storage.s3_credentials (
			service_account_id, access_key_id, encrypted_secret_key
		) VALUES ($1, $2, $3)
	`
	_, err = kernel.DB().Exec(ctx, insertCredentialSQL, parsedID, accessKeyID, encryptedSecretKey)
	if err != nil {
		t.Fatalf("failed to insert s3 credential: %v", err)
	}

	return parsedID, accessKeyID, secretAccessKey
}
