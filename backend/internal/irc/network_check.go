package irc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/lrstanley/girc"
)

// Transport failures are retried unless they identify a configuration that
// cannot succeed without a change. DNS and connection timeouts can recover.
func terminalConnectionFailure(err error) bool {
	var unknown x509.UnknownAuthorityError
	var hostname x509.HostnameError
	var invalid x509.CertificateInvalidError
	var record tls.RecordHeaderError
	var config *girc.InvalidConfigError
	return errors.Is(err, errPrivateDestination) || errors.Is(err, errWebIRCPrivateOnly) || errors.As(err, &unknown) ||
		errors.As(err, &hostname) || errors.As(err, &invalid) ||
		errors.As(err, &record) || errors.As(err, &config)
}

// CheckNetwork checks only the address and TLS. It never sends IRC credentials
// or creates a saved network. Use the same DNS pinning and address policy as IRC.
func (m *Manager) CheckNetwork(ctx context.Context, host string, port int, secure bool) error {
	if host == "" || !clean(host, 253) || strings.ContainsAny(host, " /\\") || port < 1 || port > 65535 {
		return fmt.Errorf("enter a valid server address and port")
	}
	if !secure && !m.options.AllowPrivate {
		return fmt.Errorf("TLS is required")
	}
	select {
	case m.checkSlots <- struct{}{}:
		defer func() { <-m.checkSlots }()
	default:
		return fmt.Errorf("server verification is busy; try again shortly")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	conn, err := (safeDialer{ctx: ctx, allowPrivate: m.options.AllowPrivate}).Dial("tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return errors.New(connectionFailure(err))
	}
	defer conn.Close()
	if secure {
		cfg := &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}
		if m.options.TLSConfig != nil {
			cfg = m.options.TLSConfig.Clone()
			cfg.ServerName = host
		}
		if err = tls.Client(conn, cfg).HandshakeContext(ctx); err != nil {
			return errors.New(connectionFailure(err))
		}
	}
	return nil
}

func connectionFailure(err error) string {
	var dns *net.DNSError
	var network net.Error
	var unknown x509.UnknownAuthorityError
	var hostname x509.HostnameError
	var invalid x509.CertificateInvalidError
	var record tls.RecordHeaderError
	switch {
	case errors.Is(err, context.DeadlineExceeded) || errors.As(err, &network) && network.Timeout():
		return "Connection timed out. Check the server address, port and TLS setting."
	case errors.As(err, &dns):
		return "The server address could not be found. Check the hostname."
	case errors.Is(err, syscall.ECONNREFUSED):
		return "Connection refused. Check the server address and port."
	case errors.As(err, &unknown), errors.As(err, &hostname), errors.As(err, &invalid):
		return "The server's TLS certificate could not be verified. Check the hostname and certificate."
	case errors.As(err, &record):
		return "The server did not accept TLS. Check the port and TLS setting."
	default:
		return "Could not connect to the IRC server: " + err.Error()
	}
}
