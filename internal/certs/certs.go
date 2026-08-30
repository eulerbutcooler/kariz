package certs

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/providers/dns/cloudflare"
	"github.com/go-acme/lego/v4/registration"
)

type acmeUser struct {
	email        string
	registration *registration.Resource
	key          crypto.PrivateKey
}

func (u *acmeUser) GetEmail() string                        { return u.email }
func (u *acmeUser) GetRegistration() *registration.Resource { return u.registration }
func (u *acmeUser) GetPrivateKey() crypto.PrivateKey        { return u.key }

type Manager struct {
	domain   string // "surang.xyz"
	email    string // ACME account contact
	cfToken  string // Cloudflare API token (Zone:DNS:Edit)
	cacheDir string // where cert.pem/key.pem persist
	log      *slog.Logger

	mu   sync.RWMutex
	cert *tls.Certificate
}

func New(domain, email, cfToken, cacheDir string, log *slog.Logger) (*Manager, error) {
	m := &Manager{domain: domain, email: email, cfToken: cfToken, cacheDir: cacheDir, log: log}
	cert, err := m.loadOrObtain()
	if err != nil {
		return nil, err
	}
	m.cert = cert
	go m.renewLoop()
	return m, nil
}

func (m *Manager) GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cert, nil
}

func (m *Manager) loadOrObtain() (*tls.Certificate, error) {
	certPath := filepath.Join(m.cacheDir, "wildcard.pem")
	keyPath := filepath.Join(m.cacheDir, "wildcard.key")

	if cert, err := tls.LoadX509KeyPair(certPath, keyPath); err == nil {
		if leaf, err := x509.ParseCertificate(cert.Certificate[0]); err == nil &&
			time.Until(leaf.NotAfter) > 30*24*time.Hour {
			return &cert, nil
		}
		m.log.Info("certificate near expiry — renewing")
	} else {
		m.log.Info("no cached certificate — obtaining from LetsEncrypt (DNS-01, takes ~1-2 min)")
	}

	res, err := m.obtain()
	if err != nil {
		if cert, pairErr := tls.LoadX509KeyPair(certPath, keyPath); pairErr == nil {
			m.log.Error("renewal failed — serving cached cert", "err", err)
			return &cert, nil
		}
		return nil, err
	}

	if err := os.MkdirAll(m.cacheDir, 0o700); err != nil {
		return nil, fmt.Errorf("certs: cache dir: %w", err)
	}
	if err := os.WriteFile(certPath, res.Certificate, 0o600); err != nil {
		return nil, fmt.Errorf("certs: write cert: %w", err)
	}
	if err := os.WriteFile(keyPath, res.PrivateKey, 0o600); err != nil {
		return nil, fmt.Errorf("certs: write key: %w", err)
	}

	cert, err := tls.X509KeyPair(res.Certificate, res.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("certs: parse: %w", err)
	}
	return &cert, nil
}

func (m *Manager) obtain() (*certificate.Resource, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("certs: account key: %w", err)
	}
	user := &acmeUser{email: m.email, key: key}

	cfg := lego.NewConfig(user)
	cfg.CADirURL = lego.LEDirectoryProduction
	client, err := lego.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("certs: lego client: %w", err)
	}

	prov, err := cloudflare.NewDNSProviderConfig(&cloudflare.Config{
		AuthToken:          m.cfToken,
		PropagationTimeout: 5 * time.Minute,
		PollingInterval:    5 * time.Second,
		TTL:                120,
	})
	if err != nil {
		return nil, fmt.Errorf("certs: cloudflare provider: %w", err)
	}

	if err := client.Challenge.SetDNS01Provider(prov); err != nil {
		return nil, fmt.Errorf("certs: dns01: %w", err)
	}

	reg, err := client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
	if err != nil {
		return nil, fmt.Errorf("certs: acme register: %w", err)
	}
	user.registration = reg

	res, err := client.Certificate.Obtain(certificate.ObtainRequest{
		Domains: []string{m.domain, "*." + m.domain},
		Bundle:  true,
	})
	if err != nil {
		return nil, fmt.Errorf("certs: obtain: %w", err)
	}
	return res, nil
}

func (m *Manager) renewLoop() {
	for {
		time.Sleep(24 * time.Hour)
		cert, err := m.loadOrObtain()
		if err != nil {
			m.log.Error("cert renewal failed", "err", err)
			continue
		}
		m.mu.Lock()
		m.cert = cert
		m.mu.Unlock()
	}
}
