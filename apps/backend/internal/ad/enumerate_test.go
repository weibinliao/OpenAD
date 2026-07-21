package ad

import (
	"testing"

	ldaplib "github.com/go-ldap/ldap/v3"
)

func TestSyncUserFromEntryIncludesIdentityFieldsMatchingRealtimeUser(t *testing.T) {
	entry := ldaplib.NewEntry("CN=Alice,OU=Users,DC=example,DC=com", map[string][]string{
		"sAMAccountName":     {"alice"},
		"userPrincipalName":  {"alice@example.com"},
		"displayName":        {"Alice Example"},
		"mail":               {"alice@example.com"},
		"givenName":          {"Alice"},
		"sn":                 {"Example"},
		"department":         {"Finance"},
		"division":           {"Corporate"},
		"userAccountControl": {"512"},
	})

	user := syncUserFromEntry(entry)

	if user.FirstName != "Alice" {
		t.Fatalf("FirstName = %q, want Alice", user.FirstName)
	}
	if user.LastName != "Example" {
		t.Fatalf("LastName = %q, want Example", user.LastName)
	}
	if user.Division != "Corporate" {
		t.Fatalf("Division = %q, want Corporate", user.Division)
	}
	if user.Domain != "EXAMPLE.COM" {
		t.Fatalf("Domain = %q, want EXAMPLE.COM", user.Domain)
	}
}

func TestAccountEnabledFromUAC(t *testing.T) {
	testCases := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "normal enabled account", value: "512", want: true},
		{name: "disabled account", value: "514", want: false},
		{name: "disabled with password never expires", value: "66050", want: false},
		{name: "enabled with password never expires", value: "66048", want: true},
		{name: "only disable bit", value: "2", want: false},
		{name: "empty value treated as enabled", value: "", want: true},
		{name: "garbage value treated as enabled", value: "not-a-number", want: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := accountEnabledFromUAC(testCase.value); got != testCase.want {
				t.Fatalf("accountEnabledFromUAC(%q) = %v, want %v", testCase.value, got, testCase.want)
			}
		})
	}
}

func TestGroupScopeFromGroupType(t *testing.T) {
	testCases := []struct {
		name  string
		value string
		want  string
	}{
		{name: "global security group", value: "-2147483646", want: "global"},
		{name: "domain local security group", value: "-2147483644", want: "domainlocal"},
		{name: "universal security group", value: "-2147483640", want: "universal"},
		{name: "global distribution group", value: "2", want: "global"},
		{name: "domain local distribution group", value: "4", want: "domainlocal"},
		{name: "universal distribution group", value: "8", want: "universal"},
		{name: "builtin local group", value: "-2147483643", want: "domainlocal"},
		{name: "builtin-only flag has no mapped scope", value: "-2147483647", want: ""},
		{name: "empty value", value: "", want: ""},
		{name: "garbage value", value: "abc", want: ""},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := groupScopeFromGroupType(testCase.value); got != testCase.want {
				t.Fatalf("groupScopeFromGroupType(%q) = %q, want %q", testCase.value, got, testCase.want)
			}
		})
	}
}
