package ad

import "testing"

func TestDNToDNSDomain(t *testing.T) {
	cases := []struct {
		dn   string
		want string
	}{
		{"DC=example,DC=com", "example.com"},
		{"dc=example,dc=net", "example.net"},
		{"OU=Users,DC=example,DC=com", "example.com"},
		{"", ""},
		{"CN=Config", ""},
	}
	for _, c := range cases {
		if got := dnToDNSDomain(c.dn); got != c.want {
			t.Errorf("dnToDNSDomain(%q) = %q, want %q", c.dn, got, c.want)
		}
	}
}

func TestNormalizeBindUser(t *testing.T) {
	cases := []struct {
		user, domain string
		want         string
		normalized   bool
	}{
		{"alice", "example.com", "alice@example.com", true},
		{"EXAMPLE\\alice", "example.com", "EXAMPLE\\alice", false},
		{"alice@example.com", "example.com", "alice@example.com", false},
		{"alice", "", "alice", false},
		{"  sam  ", "example.net", "sam@example.net", true},
	}
	for _, c := range cases {
		got, normalized := NormalizeBindUser(c.user, c.domain)
		if got != c.want || normalized != c.normalized {
			t.Errorf("NormalizeBindUser(%q, %q) = (%q, %v), want (%q, %v)", c.user, c.domain, got, normalized, c.want, c.normalized)
		}
	}
}
