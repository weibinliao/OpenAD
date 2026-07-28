package ad

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"testing"

	ber "github.com/go-asn1-ber/asn1-ber"
	ldaplib "github.com/go-ldap/ldap/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeServerURL(t *testing.T) {
	assert.Equal(t, "ldap://directory.example.com", normalizeServerURL("directory.example.com"))
	assert.Equal(t, "ldap://directory.example.com", normalizeServerURL(" ldap://directory.example.com "))
	assert.Equal(t, "ldaps://directory.example.com", normalizeServerURL("ldaps://directory.example.com"))
}

func TestNewADClientAndSearchUser(t *testing.T) {
	userEntry := ldaplib.NewEntry("CN=Alice,OU=Users,DC=example,DC=com", map[string][]string{
		"sAMAccountName":    {"alice"},
		"displayName":       {"Alice Example"},
		"mail":              {"alice@example.com"},
		"givenName":         {"Alice"},
		"sn":                {"Example"},
		"department":        {"Finance"},
		"division":          {"Corporate"},
		"userPrincipalName": {"alice@example.com"},
		"memberOf":          {"CN=Finance,OU=Groups,DC=example,DC=com"},
	})
	directGroupEntry := ldaplib.NewEntry("CN=Finance,OU=Groups,DC=example,DC=com", map[string][]string{
		"objectClass":    {"top", "group"},
		"cn":             {"Finance"},
		"displayName":    {"Finance Team"},
		"sAMAccountName": {"Finance"},
	})
	nestedGroupEntry := ldaplib.NewEntry("CN=All Staff,OU=Groups,DC=example,DC=com", map[string][]string{
		"objectClass":    {"top", "group"},
		"cn":             {"All Staff"},
		"displayName":    {"All Staff"},
		"sAMAccountName": {"AllStaff"},
	})
	server := newLDAPTestServer(t, ldapTestServerOptions{
		searchResponses: [][]*ldaplib.Entry{
			{userEntry},
			{directGroupEntry, nestedGroupEntry},
		},
	})

	client, err := NewADClient(server.URL(), "DC=example,DC=com", "svc-reader", "secret")
	require.NoError(t, err)
	t.Cleanup(client.Close)

	user, err := client.SearchUser("alice")
	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, "CN=Alice,OU=Users,DC=example,DC=com", user.DN)
	assert.Equal(t, "alice", user.Username)
	assert.Equal(t, "Alice Example", user.DisplayName)
	assert.Equal(t, "alice@example.com", user.Email)
	assert.Equal(t, "Alice", user.FirstName)
	assert.Equal(t, "Example", user.LastName)
	assert.Equal(t, "Finance", user.Department)
	assert.Equal(t, "Corporate", user.Division)
	assert.Equal(t, "EXAMPLE.COM", user.Domain)
	assert.Equal(t, []string{"CN=Finance,OU=Groups,DC=example,DC=com"}, user.GroupDNs)
	assert.Equal(t, []string{"Finance Team", "All Staff"}, user.Groups)
	assert.Equal(t, []string{"svc-reader"}, server.BindUsernames())
	assert.Equal(t, []string{
		"(&(objectClass=user)(!(objectClass=computer))(sAMAccountName=alice))",
		"(&(objectClass=group)(member:1.2.840.113556.1.4.1941:=CN=Alice,OU=Users,DC=example,DC=com))",
	}, server.Filters())
}

func TestADClientSearchUserReturnsNotFound(t *testing.T) {
	server := newLDAPTestServer(t, ldapTestServerOptions{})

	client, err := NewADClient(server.URL(), "DC=example,DC=com", "svc-reader", "secret")
	require.NoError(t, err)
	t.Cleanup(client.Close)

	user, err := client.SearchUser("missing-user")

	assert.Nil(t, user)
	assert.EqualError(t, err, "user not found: missing-user")
}

func TestADClientSearchUsersUsesDefaultLimitAndReturnsUsers(t *testing.T) {
	server := newLDAPTestServer(t, ldapTestServerOptions{
		entries: []*ldaplib.Entry{
			ldaplib.NewEntry("CN=Alice,OU=Users,DC=example,DC=com", map[string][]string{
				"sAMAccountName": {"alice"},
				"displayName":    {"Alice Example"},
				"mail":           {"alice@example.com"},
			}),
			ldaplib.NewEntry("CN=Alicia,OU=Users,DC=example,DC=com", map[string][]string{
				"sAMAccountName": {"alicia"},
				"displayName":    {"Alicia Example"},
				"mail":           {"alicia@example.com"},
			}),
		},
	})

	client, err := NewADClient(server.URL(), "DC=example,DC=com", "svc-reader", "secret")
	require.NoError(t, err)
	t.Cleanup(client.Close)

	users, err := client.SearchUsers("alice", 0)
	require.NoError(t, err)
	require.Len(t, users, 2)
	assert.Equal(t, 25, server.SizeLimits()[0])
	assert.Contains(t, server.Filters()[0], "(sAMAccountName=*alice*)")
	assert.Equal(t, "alice", users[0].Username)
	assert.Equal(t, "alicia", users[1].Username)
}

func TestADClientSearchUsersRequiresQuery(t *testing.T) {
	client := &ADClient{}

	users, err := client.SearchUsers("   ", 10)

	assert.Nil(t, users)
	assert.EqualError(t, err, "query is required")
}

func TestNewADClientReturnsBindError(t *testing.T) {
	server := newLDAPTestServer(t, ldapTestServerOptions{
		bindResultCode:      ldaplib.LDAPResultInvalidCredentials,
		bindDiagnosticError: "invalid credentials",
	})

	client, err := NewADClient(server.URL(), "DC=example,DC=com", "svc-reader", "wrong-secret")

	assert.Nil(t, client)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid Credentials")
}

func TestClientConnectAndClose(t *testing.T) {
	server := newLDAPTestServer(t, ldapTestServerOptions{})
	client := NewClient(server.URL(), "DC=example,DC=com", "svc-reader", "secret")

	err := client.Connect()
	require.NoError(t, err)
	assert.NotNil(t, client.client)
	assert.Equal(t, server.URL(), client.server)
	assert.Equal(t, []string{"svc-reader"}, server.BindUsernames())

	client.Close()
	assert.NotPanics(t, func() {
		(&ADClient{}).Close()
		(&Client{}).Close()
	})
}

func TestClientConnectReturnsError(t *testing.T) {
	server := newLDAPTestServer(t, ldapTestServerOptions{
		bindResultCode:      ldaplib.LDAPResultInvalidCredentials,
		bindDiagnosticError: "invalid credentials",
	})
	client := NewClient(server.URL(), "DC=example,DC=com", "svc-reader", "wrong-secret")

	err := client.Connect()

	assert.Error(t, err)
	assert.Nil(t, client.client)
}

func TestADClientContextAwareLookupsStopBeforeNetworkIOWhenCanceled(t *testing.T) {
	tests := map[string]func(context.Context, *ADClient) error{
		"group with members": func(ctx context.Context, client *ADClient) error {
			group, err := client.GetGroup(ctx, "CN=Large Group,OU=Groups,DC=example,DC=com")
			assert.Nil(t, group)
			return err
		},
		"principal": func(ctx context.Context, client *ADClient) error {
			principal, err := client.GetPrincipal(ctx, "CN=Alice,OU=Users,DC=example,DC=com")
			assert.Nil(t, principal)
			return err
		},
		"principal identifier": func(ctx context.Context, client *ADClient) error {
			principal, err := client.ResolvePrincipal(ctx, "alice")
			assert.Nil(t, principal)
			return err
		},
	}

	for name, lookup := range tests {
		t.Run(name, func(t *testing.T) {
			server := newLDAPTestServer(t, ldapTestServerOptions{})
			client, err := NewADClient(server.URL(), "DC=example,DC=com", "svc-reader", "secret")
			require.NoError(t, err)
			t.Cleanup(client.Close)

			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			err = lookup(ctx, client)

			assert.ErrorIs(t, err, context.Canceled)
			assert.Empty(t, server.Filters(), "a canceled group lookup must not traverse any members")
		})
	}
}

func TestADClientHasChildEntriesTreatsSizeLimitAsExpandable(t *testing.T) {
	server := newLDAPTestServer(t, ldapTestServerOptions{
		searchResultCode:      ldaplib.LDAPResultSizeLimitExceeded,
		searchDiagnosticError: "container is too large for the current directory limits",
	})

	client, err := NewADClient(server.URL(), "DC=example,DC=com", "svc-reader", "secret")
	require.NoError(t, err)
	t.Cleanup(client.Close)

	hasChildren, err := client.hasChildEntries("OU=Large,DC=example,DC=com")

	require.NoError(t, err)
	assert.True(t, hasChildren)
	assert.Equal(t, []int{1}, server.SizeLimits())
}

func TestADClientListTreeNodesRequestsUsersForOUBranches(t *testing.T) {
	server := newLDAPTestServer(t, ldapTestServerOptions{
		entries: []*ldaplib.Entry{
			ldaplib.NewEntry("CN=Alice,OU=IT,DC=example,DC=com", map[string][]string{
				"objectClass":    {"top", "person", "organizationalPerson", "user"},
				"sAMAccountName": {"alice"},
				"displayName":    {"Alice Example"},
			}),
			ldaplib.NewEntry("CN=Admins,OU=IT,DC=example,DC=com", map[string][]string{
				"objectClass": {"top", "group"},
				"cn":          {"Admins"},
			}),
		},
	})

	client, err := NewADClient(server.URL(), "DC=example,DC=com", "svc-reader", "secret")
	require.NoError(t, err)
	t.Cleanup(client.Close)

	listing, err := client.ListTreeNodes(context.Background(), "OU=IT,DC=example,DC=com", 50)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(listing.Nodes), 2)
	assert.Contains(t, strings.Join(server.Filters(), "\n"), "(objectClass=user)")
	assert.Equal(t, "user", listing.Nodes[0].NodeType)
	assert.Equal(t, "Alice Example", listing.Nodes[0].Name)
}

func TestSearchSingleLevelTreeNodesByFilterFollowsPagingUntilCookieEmpty(t *testing.T) {
	server := newLDAPTestServer(t, ldapTestServerOptions{
		pagedSearchPages: [][]*ldaplib.Entry{
			{ldaplib.NewEntry("OU=Applications,OU=IT,DC=example,DC=com", map[string][]string{
				"objectClass": {"top", "organizationalUnit"},
				"ou":          {"Applications"},
			})},
			{ldaplib.NewEntry("CN=Admins,OU=IT,DC=example,DC=com", map[string][]string{
				"objectClass": {"top", "group"},
				"cn":          {"Admins"},
			})},
			{ldaplib.NewEntry("CN=Alice,OU=IT,DC=example,DC=com", map[string][]string{
				"objectClass":    {"top", "person", "organizationalPerson", "user"},
				"sAMAccountName": {"alice"},
				"displayName":    {"Alice Example"},
			})},
		},
	})

	client, err := NewADClient(server.URL(), "DC=example,DC=com", "svc-reader", "secret")
	require.NoError(t, err)
	t.Cleanup(client.Close)

	result, partial, err := client.searchSingleLevelTreeNodesByFilter("OU=IT,DC=example,DC=com", 2, treeNodeFilterAll)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, partial)
	require.Len(t, result.Entries, 3)
	assert.Equal(t, []string{
		"OU=Applications,OU=IT,DC=example,DC=com",
		"CN=Admins,OU=IT,DC=example,DC=com",
		"CN=Alice,OU=IT,DC=example,DC=com",
	}, []string{result.Entries[0].DN, result.Entries[1].DN, result.Entries[2].DN})
}

func TestSearchSingleLevelTreeNodesDoesNotRetryWithNarrowerFiltersOnSizeLimit(t *testing.T) {
	server := newLDAPTestServer(t, ldapTestServerOptions{
		searchResultCode:      ldaplib.LDAPResultSizeLimitExceeded,
		searchDiagnosticError: "size limit exceeded",
	})

	client, err := NewADClient(server.URL(), "DC=example,DC=com", "svc-reader", "secret")
	require.NoError(t, err)
	t.Cleanup(client.Close)

	result, partial, warning, err := client.searchSingleLevelTreeNodes("OU=Large,DC=example,DC=com", 120)

	assert.Nil(t, result)
	assert.False(t, partial)
	assert.Empty(t, warning)
	require.Error(t, err)
	assert.Equal(t, []string{treeNodeFilterAll}, server.Filters())
}

type ldapTestServerOptions struct {
	entries               []*ldaplib.Entry
	searchResponses       [][]*ldaplib.Entry
	pagedSearchPages      [][]*ldaplib.Entry
	bindResultCode        uint16
	bindDiagnosticError   string
	searchResultCode      uint16
	searchDiagnosticError string
}

type ldapTestServer struct {
	listener net.Listener

	mu            sync.Mutex
	bindUsernames []string
	filters       []string
	sizeLimits    []int
	searchCount   int

	entries               []*ldaplib.Entry
	searchResponses       [][]*ldaplib.Entry
	pagedSearchPages      [][]*ldaplib.Entry
	bindResultCode        uint16
	bindDiagnosticError   string
	searchResultCode      uint16
	searchDiagnosticError string

	closed chan struct{}
	wg     sync.WaitGroup
}

func newLDAPTestServer(t *testing.T, options ldapTestServerOptions) *ldapTestServer {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	server := &ldapTestServer{
		listener:              listener,
		entries:               options.entries,
		searchResponses:       options.searchResponses,
		pagedSearchPages:      options.pagedSearchPages,
		bindResultCode:        options.bindResultCode,
		bindDiagnosticError:   options.bindDiagnosticError,
		searchResultCode:      options.searchResultCode,
		searchDiagnosticError: options.searchDiagnosticError,
		closed:                make(chan struct{}),
	}

	server.wg.Add(1)
	go server.serve(t)

	t.Cleanup(server.Close)
	return server
}

func (server *ldapTestServer) URL() string {
	return fmt.Sprintf("ldap://%s", server.listener.Addr().String())
}

func (server *ldapTestServer) Close() {
	select {
	case <-server.closed:
		return
	default:
		close(server.closed)
	}

	_ = server.listener.Close()
	server.wg.Wait()
}

func (server *ldapTestServer) BindUsernames() []string {
	server.mu.Lock()
	defer server.mu.Unlock()

	return append([]string(nil), server.bindUsernames...)
}

func (server *ldapTestServer) Filters() []string {
	server.mu.Lock()
	defer server.mu.Unlock()

	return append([]string(nil), server.filters...)
}

func (server *ldapTestServer) SizeLimits() []int {
	server.mu.Lock()
	defer server.mu.Unlock()

	return append([]int(nil), server.sizeLimits...)
}

func (server *ldapTestServer) serve(t *testing.T) {
	defer server.wg.Done()

	for {
		connection, err := server.listener.Accept()
		if err != nil {
			select {
			case <-server.closed:
				return
			default:
			}

			assert.NoError(t, err)
			return
		}

		server.wg.Add(1)
		go server.handleConnection(t, connection)
	}
}

func (server *ldapTestServer) handleConnection(t *testing.T, connection net.Conn) {
	defer server.wg.Done()
	defer connection.Close()

	for {
		packet, err := ber.ReadPacket(connection)
		if err != nil {
			if err == io.EOF || strings.Contains(err.Error(), "closed") {
				return
			}

			select {
			case <-server.closed:
				return
			default:
			}

			assert.NoError(t, err)
			return
		}

		if len(packet.Children) < 2 {
			continue
		}

		messageID := packetIntegerValue(packet.Children[0])
		request := packet.Children[1]

		switch request.Tag {
		case ldaplib.ApplicationBindRequest:
			server.recordBindUsername(request)
			response := newLDAPResultPacket(messageID, ldaplib.ApplicationBindResponse, server.bindResultCode, server.bindDiagnosticError)
			_, err = connection.Write(response.Bytes())
			assert.NoError(t, err)

		case ldaplib.ApplicationSearchRequest:
			server.recordSearchRequest(request)

			if server.searchResultCode != ldaplib.LDAPResultSuccess {
				response := newLDAPResultPacket(messageID, ldaplib.ApplicationSearchResultDone, server.searchResultCode, server.searchDiagnosticError)
				_, err = connection.Write(response.Bytes())
				assert.NoError(t, err)
				continue
			}

			entries, cookie := server.nextSearchResult()
			for _, entry := range entries {
				response := newSearchEntryPacket(messageID, entry)
				_, err = connection.Write(response.Bytes())
				assert.NoError(t, err)
			}

			response := newLDAPResultPacket(messageID, ldaplib.ApplicationSearchResultDone, ldaplib.LDAPResultSuccess, "")
			if cookie != nil {
				appendLDAPControls(response, []ldaplib.Control{&ldaplib.ControlPaging{Cookie: cookie}})
			}
			_, err = connection.Write(response.Bytes())
			assert.NoError(t, err)

		default:
			response := newLDAPResultPacket(messageID, ldaplib.ApplicationExtendedResponse, ldaplib.LDAPResultOperationsError, "unsupported request")
			_, err = connection.Write(response.Bytes())
			assert.NoError(t, err)
		}
	}
}

func (server *ldapTestServer) nextSearchResult() ([]*ldaplib.Entry, []byte) {
	server.mu.Lock()
	defer server.mu.Unlock()

	searchIndex := server.searchCount
	server.searchCount++

	switch {
	case len(server.searchResponses) > 0:
		if searchIndex < len(server.searchResponses) {
			return server.searchResponses[searchIndex], nil
		}
		return []*ldaplib.Entry{}, nil
	case len(server.pagedSearchPages) > 0:
		if searchIndex < len(server.pagedSearchPages) {
			var cookie []byte
			if searchIndex < len(server.pagedSearchPages)-1 {
				cookie = []byte(fmt.Sprintf("page-%d", searchIndex+1))
			}
			return server.pagedSearchPages[searchIndex], cookie
		}
		return []*ldaplib.Entry{}, nil
	default:
		return server.entries, nil
	}
}

func (server *ldapTestServer) recordBindUsername(packet *ber.Packet) {
	server.mu.Lock()
	defer server.mu.Unlock()

	if len(packet.Children) < 2 {
		return
	}

	username, _ := packet.Children[1].Value.(string)
	server.bindUsernames = append(server.bindUsernames, username)
}

func (server *ldapTestServer) recordSearchRequest(packet *ber.Packet) {
	server.mu.Lock()
	defer server.mu.Unlock()

	if len(packet.Children) < 7 {
		return
	}

	server.sizeLimits = append(server.sizeLimits, int(packetIntegerValue(packet.Children[3])))
	filter, err := ldaplib.DecompileFilter(packet.Children[6])
	if err == nil {
		server.filters = append(server.filters, filter)
	}
}

func newLDAPResultPacket(messageID int64, application uint8, resultCode uint16, diagnosticMessage string) *ber.Packet {
	packet := ber.NewSequence("LDAP Response")
	packet.AppendChild(ber.NewInteger(ber.ClassUniversal, ber.TypePrimitive, ber.TagInteger, messageID, "Message ID"))

	response := ber.Encode(ber.ClassApplication, ber.TypeConstructed, ber.Tag(application), nil, "LDAP Result")
	response.AppendChild(ber.NewInteger(ber.ClassUniversal, ber.TypePrimitive, ber.TagEnumerated, int64(resultCode), "Result Code"))
	response.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimitive, ber.TagOctetString, "", "Matched DN"))
	response.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimitive, ber.TagOctetString, diagnosticMessage, "Diagnostic Message"))

	packet.AppendChild(response)
	return packet
}

func appendLDAPControls(packet *ber.Packet, controls []ldaplib.Control) {
	if len(controls) == 0 {
		return
	}

	controlsPacket := ber.Encode(ber.ClassContext, ber.TypeConstructed, 0, nil, "Controls")
	for _, control := range controls {
		controlsPacket.AppendChild(control.Encode())
	}
	packet.AppendChild(controlsPacket)
}

func newSearchEntryPacket(messageID int64, entry *ldaplib.Entry) *ber.Packet {
	packet := ber.NewSequence("LDAP Response")
	packet.AppendChild(ber.NewInteger(ber.ClassUniversal, ber.TypePrimitive, ber.TagInteger, messageID, "Message ID"))

	searchEntry := ber.Encode(ber.ClassApplication, ber.TypeConstructed, ldaplib.ApplicationSearchResultEntry, nil, "Search Result Entry")
	searchEntry.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimitive, ber.TagOctetString, entry.DN, "Object Name"))

	attributes := ber.Encode(ber.ClassUniversal, ber.TypeConstructed, ber.TagSequence, nil, "Attributes")
	for _, attribute := range entry.Attributes {
		attributePacket := ber.Encode(ber.ClassUniversal, ber.TypeConstructed, ber.TagSequence, nil, "Attribute")
		attributePacket.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimitive, ber.TagOctetString, attribute.Name, "Attribute Name"))

		values := ber.Encode(ber.ClassUniversal, ber.TypeConstructed, ber.TagSet, nil, "Attribute Values")
		for _, value := range attribute.Values {
			values.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimitive, ber.TagOctetString, value, "Attribute Value"))
		}

		attributePacket.AppendChild(values)
		attributes.AppendChild(attributePacket)
	}

	searchEntry.AppendChild(attributes)
	packet.AppendChild(searchEntry)
	return packet
}

func packetIntegerValue(packet *ber.Packet) int64 {
	switch value := packet.Value.(type) {
	case int64:
		return value
	case uint64:
		return int64(value)
	case int32:
		return int64(value)
	case uint32:
		return int64(value)
	case int:
		return int64(value)
	case uint:
		return int64(value)
	default:
		return 0
	}
}
