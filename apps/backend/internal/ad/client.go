package ad

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/weibinliao/OpenAD/internal/models"
	"github.com/go-ldap/ldap/v3"
)

type ADClient struct {
	conn   *ldap.Conn
	baseDN string
}

type Client struct {
	server   string
	baseDN   string
	username string
	password string
	client   *ADClient
}

type TreeListing struct {
	Nodes   []models.ADTreeNode
	Partial bool
	Warning string
}

type User struct {
	DN          string   `json:"dn"`
	Username    string   `json:"username"`
	DisplayName string   `json:"display_name"`
	Email       string   `json:"email"`
	FirstName   string   `json:"first_name,omitempty"`
	LastName    string   `json:"last_name,omitempty"`
	Department  string   `json:"department,omitempty"`
	Division    string   `json:"division,omitempty"`
	Domain      string   `json:"domain,omitempty"`
	GroupDNs    []string `json:"group_dns,omitempty"`
	Groups      []string `json:"groups"`
}

const (
	treeNodeFilterAll        = "(|(objectClass=organizationalUnit)(objectClass=container)(objectClass=groupPolicyContainer)(objectClass=group)(&(objectClass=user)(!(objectClass=computer))))"
	treeNodeFilterUsersOnly  = "(|(objectClass=organizationalUnit)(objectClass=container)(objectClass=groupPolicyContainer)(&(objectClass=user)(!(objectClass=computer))))"
	treeNodeFilterStructural = "(|(objectClass=organizationalUnit)(objectClass=container)(objectClass=groupPolicyContainer))"
)

func NewADClient(server, baseDN, username, password string) (*ADClient, error) {
	conn, err := ldap.DialURL(normalizeServerURL(server))
	if err != nil {
		return nil, err
	}
	conn.SetTimeout(5 * time.Second)

	err = conn.Bind(strings.TrimSpace(username), password)
	if err != nil {
		conn.Close()
		return nil, err
	}

	return &ADClient{
		conn:   conn,
		baseDN: strings.TrimSpace(baseDN),
	}, nil
}

func NewClient(server, baseDN, username, password string) *Client {
	return &Client{
		server:   server,
		baseDN:   baseDN,
		username: username,
		password: password,
	}
}

func (c *Client) Connect() error {
	client, err := NewADClient(c.server, c.baseDN, c.username, c.password)
	if err != nil {
		return err
	}

	c.client = client
	return nil
}

func (c *ADClient) SearchUser(username string) (*User, error) {
	escapedUsername := ldap.EscapeFilter(strings.TrimSpace(username))
	searchRequest := ldap.NewSearchRequest(
		c.baseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		fmt.Sprintf("(&(objectClass=user)(!(objectClass=computer))(sAMAccountName=%s))", escapedUsername),
		[]string{"dn", "sAMAccountName", "displayName", "mail", "memberOf", "givenName", "sn", "department", "division", "userPrincipalName"},
		nil,
	)

	sr, err := c.conn.Search(searchRequest)
	if err != nil {
		return nil, err
	}

	if len(sr.Entries) == 0 {
		return nil, fmt.Errorf("user not found: %s", username)
	}

	user := userFromEntry(sr.Entries[0])
	c.enrichUserGroups(&user)
	return &user, nil
}

func (c *ADClient) SearchUsers(query string, limit int) ([]User, error) {
	trimmedQuery := strings.TrimSpace(query)
	if trimmedQuery == "" {
		return nil, fmt.Errorf("query is required")
	}

	if limit < 1 || limit > 100 {
		limit = 25
	}

	escapedQuery := ldap.EscapeFilter(trimmedQuery)
	filter := fmt.Sprintf("(&(objectClass=user)(!(objectClass=computer))(|(sAMAccountName=*%[1]s*)(displayName=*%[1]s*)(mail=*%[1]s*)))", escapedQuery)
	searchRequest := ldap.NewSearchRequest(
		c.baseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, limit, 0, false,
		filter,
		[]string{"dn", "sAMAccountName", "displayName", "mail", "memberOf", "givenName", "sn", "department", "division", "userPrincipalName"},
		nil,
	)

	sr, err := c.conn.Search(searchRequest)
	if err != nil {
		return nil, err
	}

	users := make([]User, 0, len(sr.Entries))
	for _, entry := range sr.Entries {
		user := userFromEntry(entry)
		c.enrichUserGroups(&user)
		users = append(users, user)
	}

	return users, nil
}

func (c *ADClient) SearchGroups(query string, limit int) ([]models.ADGroup, error) {
	trimmedQuery := strings.TrimSpace(query)
	if trimmedQuery == "" {
		return nil, fmt.Errorf("query is required")
	}

	if limit < 1 || limit > 100 {
		limit = 25
	}

	escapedQuery := ldap.EscapeFilter(trimmedQuery)
	filter := fmt.Sprintf("(&(objectClass=group)(|(sAMAccountName=*%[1]s*)(cn=*%[1]s*)(displayName=*%[1]s*)))", escapedQuery)
	searchRequest := ldap.NewSearchRequest(
		c.baseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, limit, 0, false,
		filter,
		[]string{"dn", "sAMAccountName", "cn", "displayName"},
		nil,
	)

	sr, err := c.conn.Search(searchRequest)
	if err != nil {
		return nil, err
	}

	groups := make([]models.ADGroup, 0, len(sr.Entries))
	for _, entry := range sr.Entries {
		groups = append(groups, groupFromEntry(entry))
	}

	return groups, nil
}

func (c *ADClient) GetGroup(ctx context.Context, distinguishedName string) (*models.ADGroup, error) {
	_ = ctx

	entry, err := c.lookupEntryByDN(distinguishedName, []string{"dn", "sAMAccountName", "cn", "displayName", "member"})
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, fmt.Errorf("group not found: %s", strings.TrimSpace(distinguishedName))
	}

	group := groupFromEntry(entry)
	memberDNs := entry.GetAttributeValues("member")
	group.Members = make([]models.ADPrincipal, 0, len(memberDNs))

	for _, memberDN := range memberDNs {
		member, err := c.GetPrincipal(ctx, memberDN)
		if err != nil {
			return nil, err
		}

		group.Members = append(group.Members, *member)
	}

	return &group, nil
}

func (c *ADClient) GetPrincipal(ctx context.Context, distinguishedName string) (*models.ADPrincipal, error) {
	_ = ctx

	entry, err := c.lookupEntryByDN(distinguishedName, []string{"dn", "objectClass", "sAMAccountName", "cn", "displayName", "objectSid", "member", "mail", "userPrincipalName", "givenName", "sn", "department", "division"})
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, fmt.Errorf("principal not found: %s", strings.TrimSpace(distinguishedName))
	}

	principal := principalFromEntry(entry)
	return &principal, nil
}

func (c *ADClient) ResolvePrincipal(ctx context.Context, identifier string) (*models.ADPrincipal, error) {
	_ = ctx

	trimmedIdentifier := strings.TrimSpace(identifier)
	if trimmedIdentifier == "" {
		return nil, nil
	}

	filter, err := buildPrincipalLookupFilter(trimmedIdentifier)
	if err != nil {
		return nil, err
	}

	searchRequest := ldap.NewSearchRequest(
		c.baseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 5, 0, false,
		filter,
		[]string{"dn", "objectClass", "sAMAccountName", "cn", "displayName", "objectSid", "member", "mail", "userPrincipalName", "givenName", "sn", "department", "division"},
		nil,
	)

	sr, err := c.conn.Search(searchRequest)
	if err != nil {
		return nil, err
	}
	if len(sr.Entries) == 0 {
		return nil, nil
	}

	principal := principalFromEntry(sr.Entries[0])
	return &principal, nil
}

func (c *ADClient) ListTreeNodes(ctx context.Context, parentDN string, limit int) (TreeListing, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return TreeListing{}, err
		}
	}

	baseDN := strings.TrimSpace(parentDN)
	if baseDN == "" {
		return TreeListing{Nodes: []models.ADTreeNode{{
			DN:          c.baseDN,
			Name:        formatDomainLabel(c.baseDN),
			NodeType:    "domain",
			HasChildren: true,
		}}}, nil
	}

	if limit < 1 || limit > 500 {
		limit = 120
	}

	parentEntry, err := c.lookupEntryByDN(baseDN, []string{"dn", "objectClass", "member"})
	if err != nil {
		return TreeListing{}, err
	}
	if parentEntry != nil && classifyTreeNodeType(parentEntry) == "group" {
		return c.listGroupMemberTreeNodes(ctx, parentEntry, limit)
	}

	sr, partial, warning, err := c.searchSingleLevelTreeNodes(baseDN, limit)
	if err != nil {
		return TreeListing{}, err
	}

	nodes := make([]models.ADTreeNode, 0, len(sr.Entries))
	for _, entry := range sr.Entries {
		nodeType := classifyTreeNodeType(entry)
		if nodeType == "" {
			continue
		}

		name := firstNonEmpty(
			entry.GetAttributeValue("ou"),
			entry.GetAttributeValue("displayName"),
			entry.GetAttributeValue("cn"),
			entry.GetAttributeValue("sAMAccountName"),
			entry.DN,
		)

		hasChildren := false
		switch nodeType {
		case "ou", "container":
			hasChildren = true
		case "group":
			hasChildren = true
		case "policy":
			hasChildren = false
		}

		nodes = append(nodes, models.ADTreeNode{
			DN:          entry.DN,
			Name:        name,
			NodeType:    nodeType,
			HasChildren: hasChildren,
		})
	}

	if limit > 0 && len(nodes) >= limit {
		partial = true
	}

	listing := TreeListing{Nodes: nodes, Partial: partial, Warning: warning}
	if listing.Partial && listing.Warning == "" {
		listing.Warning = fmt.Sprintf("Loaded %d child entries with safe paging; larger containers may be truncated.", len(nodes))
	}

	return listing, nil
}

func (c *ADClient) listGroupMemberTreeNodes(ctx context.Context, groupEntry *ldap.Entry, limit int) (TreeListing, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return TreeListing{}, err
		}
	}
	if groupEntry == nil {
		return TreeListing{Nodes: []models.ADTreeNode{}}, nil
	}

	memberDNs := groupEntry.GetAttributeValues("member")
	if len(memberDNs) == 0 {
		return TreeListing{Nodes: []models.ADTreeNode{}}, nil
	}

	trimmedMemberDNs := memberDNs
	partial := false
	unresolvedCount := 0
	if limit > 0 && len(trimmedMemberDNs) > limit {
		trimmedMemberDNs = trimmedMemberDNs[:limit]
		partial = true
	}

	nodes := make([]models.ADTreeNode, 0, len(trimmedMemberDNs))
	for _, memberDN := range trimmedMemberDNs {
		if ctx != nil {
			if err := ctx.Err(); err != nil {
				return TreeListing{}, err
			}
		}

		principal, err := c.GetPrincipal(ctx, memberDN)
		if err != nil {
			partial = true
			unresolvedCount++
			continue
		}
		if principal == nil || strings.TrimSpace(principal.DN) == "" {
			partial = true
			unresolvedCount++
			continue
		}

		nodeType := string(principal.Type)
		if nodeType == "" || nodeType == string(models.ADObjectTypeUnknown) {
			partial = true
			unresolvedCount++
			continue
		}

		nodes = append(nodes, models.ADTreeNode{
			DN:          principal.DN,
			Name:        firstNonEmpty(principal.Name, principal.SAMAccountName, principal.DN),
			NodeType:    nodeType,
			HasChildren: principal.Type == models.ADObjectTypeGroup,
		})
	}

	warning := ""
	if unresolvedCount > 0 {
		warning = fmt.Sprintf("Loaded direct group members with %d unresolved entries.", unresolvedCount)
	}
	if limit > 0 && len(memberDNs) > limit {
		if warning != "" {
			warning += " "
		}
		warning += fmt.Sprintf("Showing the first %d direct members for this group.", limit)
	}

	return TreeListing{Nodes: nodes, Partial: partial, Warning: warning}, nil
}

func (c *ADClient) searchSingleLevelTreeNodes(baseDN string, limit int) (*ldap.SearchResult, bool, string, error) {
	sr, partial, err := c.searchSingleLevelTreeNodesByFilter(baseDN, limit, treeNodeFilterAll)
	if err != nil {
		return nil, partial, "", err
	}
	return sr, partial, "", nil
}

func (c *ADClient) searchSingleLevelTreeNodesByFilter(baseDN string, limit int, filter string) (*ldap.SearchResult, bool, error) {
	attributes := []string{"dn", "ou", "cn", "displayName", "sAMAccountName", "objectClass"}

	pageSize := uint32(100)
	if limit > 0 && limit < 100 {
		pageSize = uint32(limit)
	}
	if pageSize > 500 {
		pageSize = 500
	}

	entries := make([]*ldap.Entry, 0)
	control := ldap.NewControlPaging(pageSize)
	partial := false

	for {
		searchRequest := ldap.NewSearchRequest(
			baseDN,
			ldap.ScopeSingleLevel, ldap.NeverDerefAliases, 0, 0, false,
			filter,
			attributes,
			[]ldap.Control{control},
		)

		sr, err := c.conn.Search(searchRequest)
		if err != nil {
			if isTreeSizeLimitError(err) && sr != nil && len(sr.Entries) > 0 {
				entries = append(entries, sr.Entries...)
				partial = true
				break
			}
			if isTreeSizeLimitError(err) && len(entries) > 0 {
				partial = true
				break
			}
			return nil, partial, err
		}

		if sr == nil {
			break
		}

		entries = append(entries, sr.Entries...)

		pagingControl := ldap.FindControl(sr.Controls, ldap.ControlTypePaging)
		if pagingControl == nil {
			break
		}

		typedPagingControl, ok := pagingControl.(*ldap.ControlPaging)
		if !ok {
			break
		}

		cookie := typedPagingControl.Cookie
		if len(cookie) == 0 {
			break
		}
		control.SetCookie(cookie)
	}

	return &ldap.SearchResult{Entries: entries}, partial, nil
}

func (c *ADClient) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}

func (c *Client) Close() {
	if c.client != nil {
		c.client.Close()
	}
}

func (c *ADClient) hasChildEntries(distinguishedName string) (bool, error) {
	searchRequest := ldap.NewSearchRequest(
		strings.TrimSpace(distinguishedName),
		ldap.ScopeSingleLevel, ldap.NeverDerefAliases, 1, 0, false,
		"(objectClass=*)",
		[]string{"dn"},
		[]ldap.Control{ldap.NewControlPaging(1)},
	)

	sr, err := c.conn.Search(searchRequest)
	if err != nil {
		if isTreeSizeLimitError(err) {
			return true, nil
		}
		return false, err
	}

	return len(sr.Entries) > 0, nil
}

func isTreeSizeLimitError(err error) bool {
	if err == nil {
		return false
	}

	var ldapErr *ldap.Error
	if errors.As(err, &ldapErr) {
		if ldapErr.ResultCode == ldap.LDAPResultSizeLimitExceeded || ldapErr.ResultCode == ldap.LDAPResultAdminLimitExceeded {
			return true
		}
	}

	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "size limit exceeded") ||
		strings.Contains(lower, "admin limit exceeded") ||
		strings.Contains(lower, "administrative limit exceeded") ||
		strings.Contains(lower, "container is too large")
}

func normalizeServerURL(server string) string {
	trimmedServer := strings.TrimSpace(server)
	if strings.HasPrefix(trimmedServer, "ldap://") || strings.HasPrefix(trimmedServer, "ldaps://") {
		return trimmedServer
	}

	return fmt.Sprintf("ldap://%s", trimmedServer)
}

func userFromEntry(entry *ldap.Entry) User {
	userPrincipalName := entry.GetAttributeValue("userPrincipalName")
	groupDNs := entry.GetAttributeValues("memberOf")
	return User{
		DN:          entry.DN,
		Username:    entry.GetAttributeValue("sAMAccountName"),
		DisplayName: entry.GetAttributeValue("displayName"),
		Email:       entry.GetAttributeValue("mail"),
		FirstName:   entry.GetAttributeValue("givenName"),
		LastName:    entry.GetAttributeValue("sn"),
		Department:  entry.GetAttributeValue("department"),
		Division:    entry.GetAttributeValue("division"),
		Domain:      domainFromEntry(entry.DN, userPrincipalName),
		GroupDNs:    groupDNs,
		Groups:      groupLabelsFromDNs(groupDNs),
	}
}

func (c *ADClient) enrichUserGroups(user *User) {
	if c == nil || c.conn == nil || user == nil {
		return
	}
	if strings.TrimSpace(user.DN) == "" || len(user.GroupDNs) == 0 {
		return
	}

	groupNames, err := c.searchEffectiveGroupNames(user.DN)
	if err != nil || len(groupNames) == 0 {
		return
	}
	user.Groups = groupNames
}

func (c *ADClient) searchEffectiveGroupNames(userDN string) ([]string, error) {
	escapedUserDN := ldap.EscapeFilter(strings.TrimSpace(userDN))
	filter := fmt.Sprintf("(&(objectClass=group)(member:1.2.840.113556.1.4.1941:=%s))", escapedUserDN)
	attributes := []string{"dn", "cn", "displayName", "sAMAccountName"}

	groupNames := make([]string, 0)
	seen := make(map[string]struct{})
	err := c.enumeratePaged(context.Background(), filter, attributes, 500, func(entry *ldap.Entry) error {
		name := firstNonEmpty(entry.GetAttributeValue("displayName"), entry.GetAttributeValue("cn"), entry.GetAttributeValue("sAMAccountName"), groupLabelFromDN(entry.DN))
		key := strings.ToLower(strings.TrimSpace(name))
		if key == "" {
			return nil
		}
		if _, found := seen[key]; found {
			return nil
		}
		seen[key] = struct{}{}
		groupNames = append(groupNames, name)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return groupNames, nil
}

func groupLabelsFromDNs(groupDNs []string) []string {
	labels := make([]string, 0, len(groupDNs))
	seen := make(map[string]struct{}, len(groupDNs))
	for _, groupDN := range groupDNs {
		label := groupLabelFromDN(groupDN)
		key := strings.ToLower(strings.TrimSpace(label))
		if key == "" {
			continue
		}
		if _, found := seen[key]; found {
			continue
		}
		seen[key] = struct{}{}
		labels = append(labels, label)
	}
	return labels
}

func groupLabelFromDN(groupDN string) string {
	trimmedDN := strings.TrimSpace(groupDN)
	if trimmedDN == "" {
		return ""
	}

	firstPart := strings.Split(trimmedDN, ",")[0]
	if index := strings.Index(firstPart, "="); index >= 0 && index+1 < len(firstPart) {
		return strings.TrimSpace(firstPart[index+1:])
	}
	return trimmedDN
}

func domainFromEntry(distinguishedName, userPrincipalName string) string {
	if at := strings.LastIndex(strings.TrimSpace(userPrincipalName), "@"); at >= 0 && at < len(strings.TrimSpace(userPrincipalName))-1 {
		return strings.ToUpper(strings.TrimSpace(userPrincipalName[at+1:]))
	}

	return strings.ToUpper(formatDomainLabel(distinguishedName))
}

func groupFromEntry(entry *ldap.Entry) models.ADGroup {
	return models.ADGroup{
		DN:   entry.DN,
		Name: firstNonEmpty(entry.GetAttributeValue("displayName"), entry.GetAttributeValue("cn"), entry.GetAttributeValue("sAMAccountName")),
	}
}

func principalFromEntry(entry *ldap.Entry) models.ADPrincipal {
	principalType := models.ADObjectTypeUnknown
	for _, objectClass := range entry.GetAttributeValues("objectClass") {
		switch strings.ToLower(objectClass) {
		case "group":
			principalType = models.ADObjectTypeGroup
		case "user":
			if principalType != models.ADObjectTypeGroup {
				principalType = models.ADObjectTypeUser
			}
		}
	}

	if principalType == models.ADObjectTypeUnknown {
		if len(entry.GetAttributeValues("member")) > 0 {
			principalType = models.ADObjectTypeGroup
		} else if entry.GetAttributeValue("sAMAccountName") != "" || entry.GetAttributeValue("mail") != "" {
			principalType = models.ADObjectTypeUser
		}
	}

	userPrincipalName := entry.GetAttributeValue("userPrincipalName")
	return models.ADPrincipal{
		DN:             entry.DN,
		Name:           firstNonEmpty(entry.GetAttributeValue("displayName"), entry.GetAttributeValue("cn"), entry.GetAttributeValue("name")),
		SAMAccountName: entry.GetAttributeValue("sAMAccountName"),
		SID:            sidBytesToString(entry.GetRawAttributeValue("objectSid")),
		Email:          entry.GetAttributeValue("mail"),
		FirstName:      entry.GetAttributeValue("givenName"),
		LastName:       entry.GetAttributeValue("sn"),
		Department:     entry.GetAttributeValue("department"),
		Division:       entry.GetAttributeValue("division"),
		Domain:         domainFromEntry(entry.DN, userPrincipalName),
		Type:           principalType,
	}
}

func classifyTreeNodeType(entry *ldap.Entry) string {
	objectClasses := entry.GetAttributeValues("objectClass")
	for _, objectClass := range objectClasses {
		switch strings.ToLower(objectClass) {
		case "domaindns":
			return "domain"
		case "organizationalunit":
			return "ou"
		case "container":
			return "container"
		case "grouppolicycontainer":
			return "policy"
		case "group":
			return "group"
		case "user":
			return "user"
		}
	}

	return ""
}

func formatDomainLabel(distinguishedName string) string {
	parts := strings.Split(distinguishedName, ",")
	segments := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if !strings.HasPrefix(strings.ToUpper(trimmed), "DC=") {
			continue
		}

		segments = append(segments, strings.TrimPrefix(trimmed, "DC="))
	}

	if len(segments) == 0 {
		return distinguishedName
	}

	return strings.Join(segments, ".")
}

func (c *ADClient) lookupEntryByDN(distinguishedName string, attributes []string) (*ldap.Entry, error) {
	escapedDN := ldap.EscapeFilter(strings.TrimSpace(distinguishedName))
	searchRequest := ldap.NewSearchRequest(
		c.baseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 1, 0, false,
		fmt.Sprintf("(distinguishedName=%s)", escapedDN),
		attributes,
		nil,
	)

	sr, err := c.conn.Search(searchRequest)
	if err != nil {
		return nil, err
	}

	if len(sr.Entries) == 0 {
		return nil, nil
	}

	return sr.Entries[0], nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}

	return ""
}

func buildPrincipalLookupFilter(identifier string) (string, error) {
	trimmed := strings.TrimSpace(identifier)
	if trimmed == "" {
		return "(objectClass=*)", nil
	}

	if isSIDString(trimmed) {
		sidFilter, err := sidStringToLDAPFilter(trimmed)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("(&(objectClass=*)(objectSid=%s))", sidFilter), nil
	}

	shortName := trimmed
	if index := strings.LastIndex(trimmed, `\`); index >= 0 && index < len(trimmed)-1 {
		shortName = trimmed[index+1:]
	} else if index := strings.LastIndex(trimmed, "@"); index > 0 {
		shortName = trimmed[:index]
	}

	escapedIdentifier := ldap.EscapeFilter(trimmed)
	escapedShortName := ldap.EscapeFilter(shortName)
	return fmt.Sprintf(
		"(&(objectClass=*)(|(sAMAccountName=%[1]s)(userPrincipalName=%[1]s)(distinguishedName=%[1]s)(cn=%[1]s)(displayName=%[1]s)(sAMAccountName=%[2]s)(cn=%[2]s)(displayName=%[2]s)))",
		escapedIdentifier,
		escapedShortName,
	), nil
}

func isSIDString(value string) bool {
	trimmed := strings.TrimSpace(value)
	return strings.HasPrefix(strings.ToUpper(trimmed), "S-1-")
}

func sidStringToLDAPFilter(value string) (string, error) {
	raw, err := sidStringToBytes(value)
	if err != nil {
		return "", err
	}

	var builder strings.Builder
	for _, b := range raw {
		builder.WriteString(fmt.Sprintf(`\%02X`, b))
	}
	return builder.String(), nil
}

func sidStringToBytes(value string) ([]byte, error) {
	parts := strings.Split(strings.TrimSpace(value), "-")
	if len(parts) < 3 || !strings.EqualFold(parts[0], "S") {
		return nil, fmt.Errorf("invalid SID: %s", value)
	}

	revision, err := strconv.ParseUint(parts[1], 10, 8)
	if err != nil {
		return nil, fmt.Errorf("invalid SID revision: %w", err)
	}

	identifierAuthority, err := strconv.ParseUint(parts[2], 10, 48)
	if err != nil {
		return nil, fmt.Errorf("invalid SID authority: %w", err)
	}

	subAuthorities := make([]uint32, 0, len(parts)-3)
	for _, part := range parts[3:] {
		subAuthority, parseErr := strconv.ParseUint(part, 10, 32)
		if parseErr != nil {
			return nil, fmt.Errorf("invalid SID sub authority: %w", parseErr)
		}
		subAuthorities = append(subAuthorities, uint32(subAuthority))
	}

	result := make([]byte, 8+len(subAuthorities)*4)
	result[0] = byte(revision)
	result[1] = byte(len(subAuthorities))
	for index := 0; index < 6; index++ {
		shift := uint(8 * (5 - index))
		result[2+index] = byte(identifierAuthority >> shift)
	}
	offset := 8
	for _, subAuthority := range subAuthorities {
		binary.LittleEndian.PutUint32(result[offset:], subAuthority)
		offset += 4
	}

	return result, nil
}

func sidBytesToString(value []byte) string {
	if len(value) < 8 {
		return ""
	}

	revision := value[0]
	subAuthorityCount := int(value[1])
	if len(value) < 8+subAuthorityCount*4 {
		return ""
	}

	var identifierAuthority uint64
	for index := 0; index < 6; index++ {
		identifierAuthority <<= 8
		identifierAuthority |= uint64(value[2+index])
	}

	parts := []string{"S", strconv.FormatUint(uint64(revision), 10), strconv.FormatUint(identifierAuthority, 10)}
	offset := 8
	for index := 0; index < subAuthorityCount; index++ {
		subAuthority := binary.LittleEndian.Uint32(value[offset : offset+4])
		parts = append(parts, strconv.FormatUint(uint64(subAuthority), 10))
		offset += 4
	}

	return strings.Join(parts, "-")
}
