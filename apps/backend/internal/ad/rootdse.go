package ad

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-ldap/ldap/v3"
)

// DiscoveryResult carries what we can learn from a domain controller's RootDSE
// so operators only need to supply server + account + password.
type DiscoveryResult struct {
	BaseDN      string `json:"base_dn"`       // defaultNamingContext, e.g. DC=example,DC=com
	DNSDomain   string `json:"dns_domain"`    // e.g. example.com
	DNSHostName string `json:"dns_host_name"` // e.g. dc01.example.com
	BindUser    string `json:"bind_user"`     // normalized bind identity actually used
	Normalized  bool   `json:"normalized"`    // true when the supplied identity was rewritten before bind
	ServerURL   string `json:"server_url"`
}

// dnToDNSDomain converts "DC=example,DC=com" to "example.com".
func dnToDNSDomain(dn string) string {
	parts := strings.Split(dn, ",")
	labels := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if len(trimmed) > 3 && strings.EqualFold(trimmed[:3], "DC=") {
			labels = append(labels, trimmed[3:])
		}
	}
	return strings.Join(labels, ".")
}

// NormalizeBindUser upgrades a bare sAMAccountName to a UPN using the
// discovered DNS domain. It also corrects the common DNS-domain backslash
// form (example.com\user) to a UPN while preserving DOMAIN\user.
func NormalizeBindUser(username, dnsDomain string) (string, bool) {
	trimmed := strings.TrimSpace(username)
	if trimmed == "" || strings.Contains(trimmed, "@") {
		return trimmed, false
	}

	if strings.Count(trimmed, "\\") == 1 {
		parts := strings.SplitN(trimmed, "\\", 2)
		domain := strings.TrimSpace(parts[0])
		account := strings.TrimSpace(parts[1])
		if domain != "" && account != "" && strings.Contains(domain, ".") {
			return account + "@" + domain, true
		}
		return trimmed, false
	}
	if strings.Contains(trimmed, "\\") {
		return trimmed, false
	}
	if strings.TrimSpace(dnsDomain) == "" {
		return trimmed, false
	}
	return trimmed + "@" + strings.TrimSpace(dnsDomain), true
}

func readRootDSE(conn *ldap.Conn) (baseDN, dnsHostName string, err error) {
	request := ldap.NewSearchRequest(
		"",
		ldap.ScopeBaseObject, ldap.NeverDerefAliases, 0, 0, false,
		"(objectClass=*)",
		[]string{"defaultNamingContext", "dnsHostName"},
		nil,
	)
	result, err := conn.Search(request)
	if err != nil {
		return "", "", err
	}
	if len(result.Entries) == 0 {
		return "", "", errors.New("RootDSE returned no entries")
	}
	entry := result.Entries[0]
	return entry.GetAttributeValue("defaultNamingContext"), entry.GetAttributeValue("dnsHostName"), nil
}

// Discover dials the server, reads the RootDSE (anonymously when the
// directory allows it, otherwise after bind), normalizes the bind user, and
// verifies the credentials. It returns everything needed to store a complete
// connection profile from just server + account + password.
func Discover(server, username, password string) (*DiscoveryResult, error) {
	serverURL := normalizeServerURL(server)
	conn, err := ldap.DialURL(serverURL)
	if err != nil {
		return nil, fmt.Errorf("cannot reach %s: %w", server, err)
	}
	defer conn.Close()
	conn.SetTimeout(5 * time.Second)

	// Most AD deployments allow anonymous RootDSE reads; try before binding so
	// bare usernames can be upgraded to UPNs.
	baseDN, dnsHostName, rootErr := readRootDSE(conn)
	dnsDomain := dnToDNSDomain(baseDN)

	bindUser, normalized := NormalizeBindUser(username, dnsDomain)
	if err := conn.Bind(bindUser, password); err != nil {
		if normalized {
			return nil, fmt.Errorf("bind failed as %s: %w (use user@example.com for a DNS domain or EXAMPLE\\user for a NetBIOS domain)", bindUser, err)
		}
		return nil, fmt.Errorf("bind failed: %w", err)
	}

	// If the anonymous read was refused, retry now that we are authenticated.
	if rootErr != nil || strings.TrimSpace(baseDN) == "" {
		baseDN, dnsHostName, err = readRootDSE(conn)
		if err != nil {
			return nil, fmt.Errorf("authenticated but could not read RootDSE: %w", err)
		}
		dnsDomain = dnToDNSDomain(baseDN)
	}

	if strings.TrimSpace(baseDN) == "" {
		return nil, errors.New("directory did not report a defaultNamingContext; specify Base DN manually")
	}

	return &DiscoveryResult{
		BaseDN:      baseDN,
		DNSDomain:   dnsDomain,
		DNSHostName: dnsHostName,
		BindUser:    bindUser,
		Normalized:  normalized,
		ServerURL:   serverURL,
	}, nil
}
