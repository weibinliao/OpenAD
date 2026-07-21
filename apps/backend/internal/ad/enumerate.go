package ad

import (
	"context"
	"strconv"

	"github.com/go-ldap/ldap/v3"
)

// SyncUser is a directory user snapshot used by the directory-sync subsystem.
type SyncUser struct {
	DN                string
	SAMAccountName    string
	UserPrincipalName string
	DisplayName       string
	Email             string
	FirstName         string
	LastName          string
	Department        string
	Division          string
	Domain            string
	SID               string
	Enabled           bool
	MemberOf          []string
}

// SyncGroup is a directory group snapshot used by the directory-sync subsystem.
type SyncGroup struct {
	DN       string
	Name     string
	SID      string
	Scope    string
	MemberOf []string
}

const (
	syncUserFilter  = "(&(objectClass=user)(!(objectClass=computer)))"
	syncGroupFilter = "(objectClass=group)"

	defaultSyncPageSize = 500

	// userAccountControl flag: account disabled.
	uacAccountDisabled = 0x2

	// groupType flags.
	groupTypeGlobal      = 0x2
	groupTypeDomainLocal = 0x4
	groupTypeUniversal   = 0x8
)

// EnumerateSyncUsers streams every user object under the client's base DN
// using LDAP simple paging, invoking fn for each user. Enumeration stops with
// the callback's error if fn fails, and honors ctx cancellation between pages.
func (c *ADClient) EnumerateSyncUsers(ctx context.Context, pageSize int, fn func(SyncUser) error) error {
	attributes := []string{
		"dn", "sAMAccountName", "userPrincipalName", "displayName", "mail",
		"givenName", "sn", "department", "division", "objectSid", "userAccountControl", "memberOf",
	}

	return c.enumeratePaged(ctx, syncUserFilter, attributes, pageSize, func(entry *ldap.Entry) error {
		return fn(syncUserFromEntry(entry))
	})
}

// EnumerateSyncGroups streams every group object under the client's base DN
// using LDAP simple paging, invoking fn for each group.
func (c *ADClient) EnumerateSyncGroups(ctx context.Context, pageSize int, fn func(SyncGroup) error) error {
	attributes := []string{"dn", "sAMAccountName", "cn", "objectSid", "groupType", "memberOf"}

	return c.enumeratePaged(ctx, syncGroupFilter, attributes, pageSize, func(entry *ldap.Entry) error {
		return fn(syncGroupFromEntry(entry))
	})
}

func (c *ADClient) enumeratePaged(ctx context.Context, filter string, attributes []string, pageSize int, handle func(*ldap.Entry) error) error {
	if pageSize < 1 {
		pageSize = defaultSyncPageSize
	}

	control := ldap.NewControlPaging(uint32(pageSize))
	for {
		if ctx != nil {
			if err := ctx.Err(); err != nil {
				return err
			}
		}

		searchRequest := ldap.NewSearchRequest(
			c.baseDN,
			ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
			filter,
			attributes,
			[]ldap.Control{control},
		)

		sr, err := c.conn.Search(searchRequest)
		if err != nil {
			return err
		}
		if sr == nil {
			return nil
		}

		for _, entry := range sr.Entries {
			if err := handle(entry); err != nil {
				return err
			}
		}

		pagingControl := ldap.FindControl(sr.Controls, ldap.ControlTypePaging)
		if pagingControl == nil {
			return nil
		}
		typedPagingControl, ok := pagingControl.(*ldap.ControlPaging)
		if !ok || len(typedPagingControl.Cookie) == 0 {
			return nil
		}
		control.SetCookie(typedPagingControl.Cookie)
	}
}

func syncUserFromEntry(entry *ldap.Entry) SyncUser {
	return SyncUser{
		DN:                entry.DN,
		SAMAccountName:    entry.GetAttributeValue("sAMAccountName"),
		UserPrincipalName: entry.GetAttributeValue("userPrincipalName"),
		DisplayName:       entry.GetAttributeValue("displayName"),
		Email:             entry.GetAttributeValue("mail"),
		FirstName:         entry.GetAttributeValue("givenName"),
		LastName:          entry.GetAttributeValue("sn"),
		Department:        entry.GetAttributeValue("department"),
		Division:          entry.GetAttributeValue("division"),
		Domain:            domainFromEntry(entry.DN, entry.GetAttributeValue("userPrincipalName")),
		SID:               sidBytesToString(entry.GetRawAttributeValue("objectSid")),
		Enabled:           accountEnabledFromUAC(entry.GetAttributeValue("userAccountControl")),
		MemberOf:          entry.GetAttributeValues("memberOf"),
	}
}

func syncGroupFromEntry(entry *ldap.Entry) SyncGroup {
	return SyncGroup{
		DN:       entry.DN,
		Name:     firstNonEmpty(entry.GetAttributeValue("cn"), entry.GetAttributeValue("sAMAccountName"), entry.DN),
		SID:      sidBytesToString(entry.GetRawAttributeValue("objectSid")),
		Scope:    groupScopeFromGroupType(entry.GetAttributeValue("groupType")),
		MemberOf: entry.GetAttributeValues("memberOf"),
	}
}

// accountEnabledFromUAC reports whether the ACCOUNTDISABLE bit (0x2) is not
// set in the userAccountControl attribute value. Missing or unparseable
// values are treated as enabled.
func accountEnabledFromUAC(value string) bool {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return true
	}

	return parsed&uacAccountDisabled == 0
}

// groupScopeFromGroupType maps the AD groupType attribute (a signed 32-bit
// flag set, often negative for security groups) to "global", "domainlocal"
// or "universal". Returns "" when the value is missing or unrecognized.
func groupScopeFromGroupType(value string) string {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return ""
	}

	flags := uint32(parsed)
	switch {
	case flags&groupTypeGlobal != 0:
		return "global"
	case flags&groupTypeDomainLocal != 0:
		return "domainlocal"
	case flags&groupTypeUniversal != 0:
		return "universal"
	default:
		return ""
	}
}
