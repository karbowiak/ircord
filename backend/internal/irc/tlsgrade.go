package irc

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"ircord/backend/internal/model"
)

// tlsGrades runs from worst to best; a grade is an index into it.
var tlsGrades = []string{"F", "D", "C", "B-", "B", "B+", "A-", "A", "A+"}

// tlsDetails describes a TLS connection and its server certificate. The grade
// is filled in when the details are read (gradeTLS), so expiry stays current.
func tlsDetails(state tls.ConnectionState) *model.TLSInfo {
	info := &model.TLSInfo{
		Version:    tls.VersionName(state.Version),
		Cipher:     tls.CipherSuiteName(state.CipherSuite),
		Verified:   len(state.VerifiedChains) > 0,
		ServerName: state.ServerName,
		GradeNotes: []string{},
	}
	// TLS 1.3 always uses ephemeral key exchange; before it, the suite says.
	info.ForwardSecrecy = state.Version >= tls.VersionTLS13 || strings.Contains(info.Cipher, "DHE_")
	defer gradeTLS(info, time.Now())
	if len(state.PeerCertificates) == 0 {
		return info
	}
	cert := state.PeerCertificates[0]
	info.CertificateSHA256 = certificateFingerprint(cert)
	info.Subject = cert.Subject.CommonName
	info.Issuer = cert.Issuer.CommonName
	if info.Issuer == "" && len(cert.Issuer.Organization) > 0 {
		info.Issuer = cert.Issuer.Organization[0]
	}
	notBefore, notAfter := cert.NotBefore.UTC(), cert.NotAfter.UTC()
	info.NotBefore, info.NotAfter = &notBefore, &notAfter
	info.SignatureAlgorithm = cert.SignatureAlgorithm.String()
	switch key := cert.PublicKey.(type) {
	case *rsa.PublicKey:
		info.KeyAlgorithm, info.KeyBits = "RSA", key.N.BitLen()
	case *ecdsa.PublicKey:
		info.KeyAlgorithm, info.KeyBits = "ECDSA", key.Curve.Params().BitSize
	case ed25519.PublicKey:
		info.KeyAlgorithm, info.KeyBits = "Ed25519", 256
	default:
		info.KeyAlgorithm = "other"
	}
	return info
}

func certificateFingerprint(cert *x509.Certificate) string {
	sum := sha256.Sum256(cert.Raw)
	return hex.EncodeToString(sum[:])
}

// gradeTLS sets a letter grade for the connection's security as of now, with
// a plain-language note for each thing that holds it back. It is a summary in
// the spirit of SSL Labs' grades, from what one client connection can see:
// the protocol and cipher it negotiated and the certificate it was shown.
func gradeTLS(info *model.TLSInfo, now time.Time) {
	score := len(tlsGrades) - 1
	notes := []string{}
	limit := func(max int, note string) {
		score = min(score, max)
		notes = append(notes, note)
	}
	const aMinus, b, bMinus, c, d, f = 6, 4, 3, 2, 1, 0
	aead := strings.Contains(info.Cipher, "_GCM_") || strings.Contains(info.Cipher, "CHACHA20")
	switch info.Version {
	case "TLS 1.3":
	case "TLS 1.2":
		limit(aMinus, "Uses TLS 1.2. TLS 1.3 is faster and removes older, weaker options.")
		if !info.ForwardSecrecy {
			limit(b, "The cipher has no forward secrecy: recorded traffic could be decrypted later if the server's key leaks.")
		}
		if !aead {
			limit(bMinus, "The cipher uses an older mode (CBC) instead of authenticated encryption.")
		}
	case "TLS 1.1", "TLS 1.0":
		limit(d, fmt.Sprintf("Uses %s, which is outdated and has known weaknesses.", info.Version))
	default:
		limit(f, "Uses an obsolete or unknown protocol version.")
	}
	switch {
	case info.KeyAlgorithm == "RSA" && info.KeyBits < 2048:
		limit(c, fmt.Sprintf("The certificate's RSA key is only %d bits; 2048 is the minimum considered safe.", info.KeyBits))
	case info.KeyAlgorithm == "ECDSA" && info.KeyBits < 256:
		limit(c, fmt.Sprintf("The certificate's ECDSA key is only %d bits.", info.KeyBits))
	case info.KeyAlgorithm == "RSA" && info.KeyBits < 3072:
		limit(len(tlsGrades)-2, fmt.Sprintf("The certificate uses a %d-bit RSA key. A larger RSA key or an ECDSA key earns A+.", info.KeyBits))
	}
	if sig := strings.ToUpper(info.SignatureAlgorithm); strings.Contains(sig, "SHA1") || strings.Contains(sig, "MD5") {
		limit(c, "The certificate is signed with an outdated hash (SHA-1 or MD5).")
	}
	if !info.Verified {
		limit(c, "The certificate isn't trusted: it may be self-signed, issued for a different name, or from an unknown issuer.")
	}
	switch {
	case info.NotAfter != nil && now.After(*info.NotAfter):
		limit(f, "The certificate has expired.")
	case info.NotBefore != nil && now.Before(*info.NotBefore):
		limit(f, "The certificate isn't valid yet.")
	case info.NotAfter != nil && info.NotAfter.Sub(now) < 14*24*time.Hour:
		days := int(info.NotAfter.Sub(now).Hours() / 24)
		limit(b, fmt.Sprintf("The certificate expires in %d %s.", days, map[bool]string{true: "day", false: "days"}[days == 1]))
	}
	info.Grade = tlsGrades[score]
	info.GradeNotes = notes
}
