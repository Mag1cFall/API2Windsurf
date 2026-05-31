package proxy

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

const (
	caCommonName = "API2Windsurf Local CA"
	caOrg        = "API2Windsurf"
)

func dataDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".api2windsurf")
}

func certDir() string { return filepath.Join(dataDir(), "ca") }

func caCertFile() string   { return filepath.Join(certDir(), "ca.pem") }
func caKeyFile() string    { return filepath.Join(certDir(), "ca.key") }
func hostCertFile() string { return filepath.Join(certDir(), "host.pem") }
func hostKeyFile() string  { return filepath.Join(certDir(), "host.key") }

func CACertPath() string { return caCertFile() }

func EnsureCertificates() (*tls.Certificate, error) {
	if err := os.MkdirAll(certDir(), 0o700); err != nil {
		return nil, fmt.Errorf("create cert dir: %w", err)
	}
	if _, err := os.Stat(caCertFile()); os.IsNotExist(err) {
		if err := createCA(); err != nil {
			return nil, fmt.Errorf("create CA: %w", err)
		}
	}
	caCert, caKey, err := loadCA()
	if err != nil {
		return nil, fmt.Errorf("load CA: %w", err)
	}
	if err := createHostCert(caCert, caKey); err != nil {
		return nil, fmt.Errorf("create host cert: %w", err)
	}
	cert, err := tls.LoadX509KeyPair(hostCertFile(), hostKeyFile())
	if err != nil {
		return nil, fmt.Errorf("load host cert: %w", err)
	}
	return &cert, nil
}

func createCA() error {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: caCommonName, Organization: []string{caOrg}},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		MaxPathLen:            1,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return err
	}
	if err := writePEM(caCertFile(), "CERTIFICATE", der, 0o644); err != nil {
		return err
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return err
	}
	return writePEM(caKeyFile(), "EC PRIVATE KEY", keyDER, 0o600)
}

func loadCA() (*x509.Certificate, *ecdsa.PrivateKey, error) {
	certPEM, err := os.ReadFile(caCertFile())
	if err != nil {
		return nil, nil, err
	}
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, nil, fmt.Errorf("decode CA cert PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, nil, err
	}
	keyPEM, err := os.ReadFile(caKeyFile())
	if err != nil {
		return nil, nil, err
	}
	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return nil, nil, fmt.Errorf("decode CA key PEM")
	}
	key, err := x509.ParseECPrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, nil, err
	}
	return cert, key, nil
}

func createHostCert(caCert *x509.Certificate, caKey *ecdsa.PrivateKey) error {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: HijackDomains[0], Organization: []string{caOrg}},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().AddDate(2, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     append([]string{}, HijackDomains...),
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, &key.PublicKey, caKey)
	if err != nil {
		return err
	}
	if err := writePEM(hostCertFile(), "CERTIFICATE", der, 0o644); err != nil {
		return err
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return err
	}
	return writePEM(hostKeyFile(), "EC PRIVATE KEY", keyDER, 0o600)
}

func writePEM(path, blockType string, der []byte, perm os.FileMode) error {
	data := pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: der})
	return os.WriteFile(path, data, perm)
}

var (
	caCheckCache bool
	caCheckAt    time.Time
)

func IsCAInstalled() bool {
	if time.Since(caCheckAt) < 30*time.Second {
		return caCheckCache
	}
	caCheckCache = checkCAInstalled()
	caCheckAt = time.Now()
	return caCheckCache
}

func InvalidateCACache() { caCheckAt = time.Time{} }

func checkCAInstalled() bool {
	if _, err := os.Stat(caCertFile()); os.IsNotExist(err) {
		return false
	}
	switch runtime.GOOS {
	case "windows":
		return runHidden("certutil", "-verifystore", "Root", caCommonName) == nil
	default:
		return localCAMatchesSystem()
	}
}

func localCAMatchesSystem() bool {
	local, err := os.ReadFile(caCertFile())
	if err != nil || len(local) == 0 {
		return false
	}
	system, err := os.ReadFile(linuxSystemCAPath())
	if err != nil || len(system) == 0 {
		return false
	}
	return string(local) == string(system)
}

func linuxSystemCAPath() string {
	return "/usr/local/share/ca-certificates/api2windsurf-ca.crt"
}

func InstallCA() error {
	if _, err := os.Stat(caCertFile()); os.IsNotExist(err) {
		return fmt.Errorf("CA cert missing; generate it first")
	}
	switch runtime.GOOS {
	case "windows":
		if err := runHidden("certutil", "-addstore", "Root", caCertFile()); err != nil {
			return fmt.Errorf("certutil add-store failed: %w", err)
		}
	default:
		data, err := os.ReadFile(caCertFile())
		if err != nil {
			return err
		}
		if err := os.WriteFile(linuxSystemCAPath(), data, 0o644); err != nil {
			return fmt.Errorf("copy CA into system store (needs privilege): %w", err)
		}
		if err := runPlain("update-ca-certificates"); err != nil {
			return fmt.Errorf("update-ca-certificates failed: %w", err)
		}
	}
	InvalidateCACache()
	return nil
}

func runPlain(name string, args ...string) error {
	return execCommand(name, args...).Run()
}
