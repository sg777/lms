package validation

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"

	"github.com/verifiable-state-chains/lms/models"
)

// SignatureVerifier provides cryptographic signature verification
type SignatureVerifier struct {
	// Trusted CA certificates for verifying attestation certificates
	trustedCAs []*x509.Certificate
}

// NewSignatureVerifier creates a new signature verifier
func NewSignatureVerifier() *SignatureVerifier {
	return &SignatureVerifier{
		trustedCAs: make([]*x509.Certificate, 0),
	}
}

// AddTrustedCA adds a trusted CA certificate
func (sv *SignatureVerifier) AddTrustedCA(cert *x509.Certificate) {
	sv.trustedCAs = append(sv.trustedCAs, cert)
}

// VerifyAttestationSignature verifies the signature on an attestation
// This is a placeholder implementation - in production, this would:
// 1. Parse the certificate from the attestation
// 2. Verify the certificate chain against trusted CAs
// 3. Extract the public key from the certificate
// 4. Verify the signature over the data using the public key
func (sv *SignatureVerifier) VerifyAttestationSignature(attestation *models.AttestationResponse) error {
	// Step 1: Decode certificate
	certValue := attestation.AttestationResponse.Certificate.Value
	certBytes, err := base64.StdEncoding.DecodeString(certValue)
	if err != nil {
		return fmt.Errorf("failed to decode certificate: %v", err)
	}

	// Step 2: Parse PEM
	block, _ := pem.Decode(certBytes)
	if block == nil {
		return fmt.Errorf("failed to parse PEM certificate")
	}

	// Step 3: Parse X.509 certificate
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		// If parsing fails, it might be a mock certificate
		// In production, this would be a hard error
		// For now, we'll just log and continue
		return fmt.Errorf("failed to parse X.509 certificate (may be mock): %v", err)
	}

	// Step 4: Verify certificate chain (if trusted CAs are configured)
	if len(sv.trustedCAs) > 0 {
		opts := x509.VerifyOptions{
			Roots: x509.NewCertPool(),
		}
		for _, ca := range sv.trustedCAs {
			opts.Roots.AddCert(ca)
		}

		_, err := cert.Verify(opts)
		if err != nil {
			return fmt.Errorf("certificate verification failed: %v", err)
		}
	}

	// Step 5: Extract public key
	pubKey := cert.PublicKey

	// Step 6: Decode signature
	sigValue := attestation.AttestationResponse.Signature.Value
	sigBytes, err := base64.StdEncoding.DecodeString(sigValue)
	if err != nil {
		return fmt.Errorf("failed to decode signature: %v", err)
	}

	// Step 7: Prepare data to verify (the chained payload)
	payload, err := attestation.GetChainedPayload()
	if err != nil {
		return fmt.Errorf("failed to get chained payload: %v", err)
	}

	// Serialize payload for signing
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %v", err)
	}

	// Step 8: Hash the data (SHA-256)
	hasher := sha256.New()
	hasher.Write(payloadJSON)
	hash := hasher.Sum(nil)

	// Step 9: Verify signature
	// Note: The actual signature algorithm depends on the certificate's public key algorithm
	// This is a simplified version - in production, you'd need to handle different algorithms
	switch pubKey.(type) {
	case *rsa.PublicKey:
		// RSA signature verification
		rsaPubKey := pubKey.(*rsa.PublicKey)
		err := rsa.VerifyPKCS1v15(rsaPubKey, crypto.SHA256, hash, sigBytes)
		if err != nil {
			return fmt.Errorf("RSA signature verification failed: %v", err)
		}
	case *ecdsa.PublicKey:
		// ECDSA signature verification
		ecdsaPubKey := pubKey.(*ecdsa.PublicKey)
		if !ecdsa.VerifyASN1(ecdsaPubKey, hash, sigBytes) {
			return fmt.Errorf("ECDSA signature verification failed")
		}
	default:
		// For mock certificates or unsupported algorithms, we'll skip verification
		// In production, this would be an error
		return fmt.Errorf("unsupported public key type or mock certificate")
	}

	return nil
}

// MockSignatureVerifier creates a verifier that always succeeds (for testing)
func MockSignatureVerifier() func(attestation *models.AttestationResponse) error {
	return func(attestation *models.AttestationResponse) error {
		// Just check signature is present and base64 decodable
		if attestation.AttestationResponse.Signature.Value == "" {
			return fmt.Errorf("signature is empty")
		}
		_, err := base64.StdEncoding.DecodeString(attestation.AttestationResponse.Signature.Value)
		if err != nil {
			return fmt.Errorf("signature is not valid base64: %v", err)
		}
		return nil
	}
}

