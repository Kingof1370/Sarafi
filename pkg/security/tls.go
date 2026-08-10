package security

import (
	"crypto/tls"
	"fmt"
	"sync"
	"time"
)

type TLSConfig struct {
	CertFile          string
	KeyFile           string
	CAFile            string
	MinVersion        string
	MaxVersion        string
	RequireClientCert bool
	CipherSuites      []string
}

type TLSManager struct {
	config     *TLSConfig
	tlsConfig  *tls.Config
	mutex      *sync.RWMutex
	lastUpdate time.Time
}

func NewTLSManager(cfg *TLSConfig) (*TLSManager, error) {
	tm := &TLSManager{
		config:    cfg,
		mutex:     &sync.RWMutex{},
		lastUpdate: time.Now(),
	}

	if err := tm.loadTLSConfig(); err != nil {
		return nil, fmt.Errorf("failed to load TLS config: %w", err)
	}

	return tm, nil
}

func (tm *TLSManager) loadTLSConfig() error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	cert, err := tls.LoadX509KeyPair(tm.config.CertFile, tm.config.KeyFile)
	if err != nil {
		return fmt.Errorf("failed to load certificate: %w", err)
	}

	minVersion := tls.VersionTLS13
	maxVersion := tls.VersionTLS13

	if tm.config.MinVersion != "" {
		if v, ok := tlsVersionMap[tm.config.MinVersion]; ok {
			minVersion = v
		}
	}

	if tm.config.MaxVersion != "" {
		if v, ok := tlsVersionMap[tm.config.MaxVersion]; ok {
			maxVersion = v
		}
	}

	cipherSuites := defaultCipherSuites
	if len(tm.config.CipherSuites) > 0 {
		cipherSuites = getCipherSuiteIDs(tm.config.CipherSuites)
	}

	tm.tlsConfig = &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:  minVersion,
		MaxVersion:  maxVersion,
		CipherSuites: cipherSuites,
		PreferServerCipherSuites: true,
		NextProtos:  []string{"h2", "http/1.1"},
		ClientAuth:  tls.NoClientCert,
	}

	if tm.config.RequireClientCert {
		tm.tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	}

	return nil
}

func (tm *TLSManager) GetTLSConfig() *tls.Config {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	if tm.tlsConfig == nil {
		return nil
	}

	return tm.tlsConfig
}

func (tm *TLSManager) ReloadCertificates() error {
	return tm.loadTLSConfig()
}

func (tm *TLSManager) RotateCertificate(newCertFile, newKeyFile string) error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	cert, err := tls.LoadX509KeyPair(newCertFile, newKeyFile)
	if err != nil {
		return fmt.Errorf("failed to load new certificate: %w", err)
	}

	tm.tlsConfig.Certificates = []tls.Certificate{cert}
	tm.lastUpdate = time.Now()

	return nil
}

var tlsVersionMap = map[string]uint16{
	"1.0": tls.VersionTLS10,
	"1.1": tls.VersionTLS11,
	"1.2": tls.VersionTLS12,
	"1.3": tls.VersionTLS13,
}

var defaultCipherSuites = []uint16{
	tls.TLS_AES_256_GCM_SHA384,
	tls.TLS_CHACHA20_POLY1305_SHA256,
	tls.TLS_AES_128_GCM_SHA256,
}

func getCipherSuiteIDs(names []string) []uint16 {
	var ids []uint16
	cipherMap := map[string]uint16{
		"TLS_AES_256_GCM_SHA384":       tls.TLS_AES_256_GCM_SHA384,
		"TLS_CHACHA20_POLY1305_SHA256": tls.TLS_CHACHA20_POLY1305_SHA256,
		"TLS_AES_128_GCM_SHA256":       tls.TLS_AES_128_GCM_SHA256,
	}

	for _, name := range names {
		if id, ok := cipherMap[name]; ok {
			ids = append(ids, id)
		}
	}

	if len(ids) == 0 {
		return defaultCipherSuites
	}

	return ids
}
