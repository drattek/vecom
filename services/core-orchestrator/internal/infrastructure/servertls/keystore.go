package servertls

import (
	"crypto/tls"
	"fmt"
	"os"
	"strings"

	"software.sslmate.com/src/go-pkcs12"
)

func LoadTLSConfig(keystorePath, keystorePassword, keystoreType string) (*tls.Config, error) {
	if strings.TrimSpace(keystorePath) == "" {
		return nil, nil
	}

	if !isSupportedKeystoreType(keystoreType) {
		return nil, fmt.Errorf("unsupported TLS keystore type %q", keystoreType)
	}

	keystoreData, err := os.ReadFile(keystorePath)
	if err != nil {
		return nil, fmt.Errorf("read TLS keystore: %w", err)
	}

	privateKey, certificate, caCertificates, err := pkcs12.DecodeChain(keystoreData, keystorePassword)
	if err != nil {
		return nil, fmt.Errorf("decode TLS keystore: %w", err)
	}

	tlsCertificate := tls.Certificate{
		Certificate: make([][]byte, 0, 1+len(caCertificates)),
		PrivateKey:  privateKey,
		Leaf:        certificate,
	}
	tlsCertificate.Certificate = append(tlsCertificate.Certificate, certificate.Raw)
	for _, caCertificate := range caCertificates {
		tlsCertificate.Certificate = append(tlsCertificate.Certificate, caCertificate.Raw)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{tlsCertificate},
		MinVersion:   tls.VersionTLS12,
	}, nil
}

func isSupportedKeystoreType(keystoreType string) bool {
	switch strings.ToUpper(strings.TrimSpace(keystoreType)) {
	case "", "PKCS12", "P12", "PFX":
		return true
	default:
		return false
	}
}
