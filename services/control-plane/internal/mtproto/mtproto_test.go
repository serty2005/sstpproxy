package mtproto

import "testing"

func TestValidateSecretAcceptsHexFakeTLS(t *testing.T) {
	secret := "ee2bd9c81483eff4ff6a923b3a4c603963676f6f676c652e636f6d"

	if err := ValidateSecret(secret); err != nil {
		t.Fatalf("ValidateSecret() вернул ошибку для корректного hex secret: %v", err)
	}
}

func TestValidateSecretAcceptsUppercaseHexFakeTLS(t *testing.T) {
	secret := "EE2BD9C81483EFF4FF6A923B3A4C603963676F6F676C652E636F6D"

	if err := ValidateSecret(secret); err != nil {
		t.Fatalf("ValidateSecret() вернул ошибку для корректного uppercase hex secret: %v", err)
	}
}

func TestValidateSecretAcceptsBase64FakeTLS(t *testing.T) {
	secret := "7ivZyBSD7_T_apI7OkxgOWNnb29nbGUuY29t"

	if err := ValidateSecret(secret); err != nil {
		t.Fatalf("ValidateSecret() вернул ошибку для корректного base64 secret: %v", err)
	}
}

func TestValidateSecretRejectsPlaceholder(t *testing.T) {
	if err := ValidateSecret("replace-with-mtproto-secret"); err == nil {
		t.Fatal("ValidateSecret() принял шаблонный secret")
	}
}

func TestValidateSecretRejectsLegacySecret(t *testing.T) {
	if err := ValidateSecret("00112233445566778899aabbccddeeff"); err == nil {
		t.Fatal("ValidateSecret() принял secret без FakeTLS-префикса ee")
	}
}
