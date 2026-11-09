package commands

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"strings"
	"time"

	"github.com/Syngnat/GoNavi-Plugin-Market/plugins/nginx-inspector/internal/ngxconf"
	"github.com/Syngnat/GoNavi-Plugin-Market/plugins/nginx-inspector/internal/sshx"
)

const maxCertFileBytes = 1 << 20

// CertificateInfo is one reviewed certificate file. File is the resolved
// absolute path (relative ssl_certificate values are joined to the nginx
// prefix). Readable reports whether the file could be read; ParseError is set
// when the bytes were read but were not a usable PEM certificate.
type CertificateInfo struct {
	Path           string   `json:"path"`
	File           string   `json:"file,omitempty"`
	Readable       bool     `json:"readable"`
	Reason         string   `json:"reason,omitempty"`
	ParseError     bool     `json:"parseError,omitempty"`
	ParseErrorText string   `json:"parseErrorText,omitempty"`
	CommonName     string   `json:"commonName,omitempty"`
	Domains        []string `json:"domains,omitempty"`
	Issuer         string   `json:"issuer,omitempty"`
	NotBefore      string   `json:"notBefore,omitempty"`
	NotAfter       string   `json:"notAfter,omitempty"`
	DaysLeft       int      `json:"daysLeft"`
	Expired        bool     `json:"expired"`
	ExpiringSoon   bool     `json:"expiringSoon"`
}

// CertificateReport is the certs.list response. Unreadable counts files that
// could not be read, ParseErrors counts files that were read but were not a
// certificate, and Skipped counts paths that could not be used at all
// (variables, path traversal, outside the read allowlist).
type CertificateReport struct {
	Certificates []CertificateInfo `json:"certificates"`
	ExpiringSoon int               `json:"expiringSoon"`
	Expired      int               `json:"expired"`
	Unreadable   int               `json:"unreadable"`
	ParseErrors  int               `json:"parseErrors"`
	Skipped      int               `json:"skipped"`
}

// certPathRef is one ssl_certificate value after resolution. Resolved is
// empty when the path cannot be used and Reason explains why.
type certPathRef struct {
	Raw      string
	Resolved string
	Reason   string
}

func (r *Runtime) certsList(ctx context.Context, args json.RawMessage) (any, error) {
	input, perr := decodeArgs[struct {
		ConnectionID string `json:"connectionId"`
	}](args)
	if perr != nil {
		return nil, perr
	}
	loaded, perr := r.loadTree(ctx, input.ConnectionID)
	if perr != nil {
		return nil, perr
	}
	info, perr := r.ensureInfo(ctx, input.ConnectionID)
	if perr != nil {
		return nil, perr
	}
	profile, perr := r.profile(input.ConnectionID)
	if perr != nil {
		return nil, perr
	}
	refs := collectSSLCertificates(info, loaded.directives)
	report := &CertificateReport{Certificates: []CertificateInfo{}}
	now := time.Now()
	for _, ref := range refs {
		entry := CertificateInfo{Path: ref.Raw, File: ref.Resolved}
		if ref.Reason != "" {
			entry.Readable = false
			entry.Reason = ref.Reason
			report.Skipped++
			report.Certificates = append(report.Certificates, entry)
			continue
		}
		read, perr := r.pool.ReadFile(ctx, profile, ref.Resolved, maxCertFileBytes)
		if perr != nil {
			entry.Reason = "file is not readable by the SSH user: " + perr.Message
			report.Unreadable++
			report.Certificates = append(report.Certificates, entry)
			continue
		}
		entry.Readable = true
		cert, err := sshx.ParseCertificatePEM([]byte(read.Content))
		if err != nil {
			entry.ParseError = true
			entry.ParseErrorText = err.Error()
			entry.Reason = "file is readable but does not contain a PEM X.509 certificate"
			report.ParseErrors++
			report.Certificates = append(report.Certificates, entry)
			continue
		}
		entry.CommonName = cert.Subject.CommonName
		entry.Domains = certificateDomains(cert)
		entry.Issuer = cert.Issuer.CommonName
		entry.NotBefore = cert.NotBefore.UTC().Format(time.RFC3339)
		entry.NotAfter = cert.NotAfter.UTC().Format(time.RFC3339)
		entry.DaysLeft = int(cert.NotAfter.Sub(now).Hours() / 24)
		entry.Expired = now.After(cert.NotAfter)
		entry.ExpiringSoon = !entry.Expired && entry.DaysLeft <= 30
		if entry.Expired {
			report.Expired++
		} else if entry.ExpiringSoon {
			report.ExpiringSoon++
		}
		report.Certificates = append(report.Certificates, entry)
	}
	return report, nil
}

// collectSSLCertificates walks the merged configuration and returns every
// usable ssl_certificate value in first-seen order. Recursive traversal is
// required: http-level snippets and per-server includes put the directive
// below server blocks, where a top-level scan never sees it.
func collectSSLCertificates(info *HostInfo, directives []ngxconf.Directive) []certPathRef {
	var rawPaths []string
	walkDirectives(directives, func(directive *ngxconf.Directive) {
		if directive.Name != "ssl_certificate" || len(directive.Args) == 0 {
			return
		}
		rawPaths = append(rawPaths, strings.TrimSpace(directive.Args[0].Value))
	})
	seen := map[string]bool{}
	refs := make([]certPathRef, 0, len(rawPaths))
	for _, raw := range rawPaths {
		if raw == "" {
			continue
		}
		if strings.Contains(raw, "$") {
			key := "raw:" + raw
			if seen[key] {
				continue
			}
			seen[key] = true
			refs = append(refs, certPathRef{
				Raw:    raw,
				Reason: "certificate path contains a variable and cannot be resolved",
			})
			continue
		}
		resolved, perr := allowPath(info, raw, true)
		if perr != nil {
			key := "raw:" + raw
			if seen[key] {
				continue
			}
			seen[key] = true
			refs = append(refs, certPathRef{Raw: raw, Reason: perr.Message})
			continue
		}
		if seen[resolved] {
			continue
		}
		seen[resolved] = true
		refs = append(refs, certPathRef{Raw: raw, Resolved: resolved})
	}
	return refs
}

// certificateDomains returns the SAN entries, falling back to the subject
// common name for certificates issued before SAN was mandatory.
func certificateDomains(cert *x509.Certificate) []string {
	out := make([]string, 0, len(cert.DNSNames)+len(cert.IPAddresses)+1)
	out = append(out, cert.DNSNames...)
	for _, ip := range cert.IPAddresses {
		out = append(out, ip.String())
	}
	if len(out) == 0 && cert.Subject.CommonName != "" {
		out = append(out, cert.Subject.CommonName)
	}
	return dedupe(out)
}
