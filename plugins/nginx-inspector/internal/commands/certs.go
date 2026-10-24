package commands

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/Syngnat/GoNavi-Plugin-Market/plugins/nginx-inspector/internal/ngxconf"
	"github.com/Syngnat/GoNavi-Plugin-Market/plugins/nginx-inspector/internal/sshx"
)

const maxCertFileBytes = 1 << 20

// CertificateInfo is one reviewed certificate file.
type CertificateInfo struct {
	Path         string   `json:"path"`
	Readable     bool     `json:"readable"`
	Reason       string   `json:"reason,omitempty"`
	Domains      []string `json:"domains,omitempty"`
	Issuer       string   `json:"issuer,omitempty"`
	NotBefore    string   `json:"notBefore,omitempty"`
	NotAfter     string   `json:"notAfter,omitempty"`
	DaysLeft     int      `json:"daysLeft,omitempty"`
	Expired      bool     `json:"expired"`
	ExpiringSoon bool     `json:"expiringSoon"`
}

// CertificateReport is the certs.list response.
type CertificateReport struct {
	Certificates []CertificateInfo `json:"certificates"`
	ExpiringSoon int               `json:"expiringSoon"`
	Expired      int               `json:"expired"`
	Unreadable   int               `json:"unreadable"`
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
	profile, perr := r.profile(input.ConnectionID)
	if perr != nil {
		return nil, perr
	}
	paths := dedupe(ngxconf.AllDirectiveArgs(loaded.directives, "ssl_certificate"))
	report := &CertificateReport{Certificates: []CertificateInfo{}}
	for _, certPath := range paths {
		info := CertificateInfo{Path: certPath}
		read, perr := r.pool.ReadFile(ctx, profile, certPath, maxCertFileBytes)
		if perr != nil {
			info.Readable = false
			info.Reason = "file is not readable by the SSH user"
			report.Unreadable++
			report.Certificates = append(report.Certificates, info)
			continue
		}
		cert, err := sshx.ParseCertificatePEM([]byte(read.Content))
		if err != nil {
			info.Reason = err.Error()
			report.Unreadable++
			report.Certificates = append(report.Certificates, info)
			continue
		}
		info.Readable = true
		info.Domains = cert.DNSNames
		if len(cert.IPAddresses) > 0 {
			for _, ip := range cert.IPAddresses {
				info.Domains = append(info.Domains, ip.String())
			}
		}
		if cn := strings.TrimPrefix(cert.Subject.String(), "CN="); cn != "" && len(info.Domains) == 0 {
			info.Domains = []string{cn}
		}
		info.Issuer = cert.Issuer.CommonName
		info.NotBefore = cert.NotBefore.UTC().Format(time.RFC3339)
		info.NotAfter = cert.NotAfter.UTC().Format(time.RFC3339)
		days := int(time.Until(cert.NotAfter).Hours() / 24)
		info.DaysLeft = days
		info.Expired = time.Now().After(cert.NotAfter)
		info.ExpiringSoon = !info.Expired && days <= 30
		if info.Expired {
			report.Expired++
		} else if info.ExpiringSoon {
			report.ExpiringSoon++
		}
		report.Certificates = append(report.Certificates, info)
	}
	return report, nil
}
