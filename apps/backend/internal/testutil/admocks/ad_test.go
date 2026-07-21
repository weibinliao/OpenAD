package admocks

import (
	"errors"
	"testing"

	"github.com/weibinliao/OpenAD/internal/ad"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestADServiceReturnsReusableClients(t *testing.T) {
	service := &ADService{}

	connectionClient := service.NewConnectionClient("", "", "", "")
	require.NoError(t, connectionClient.Connect())
	connectionClient.Close()

	userClient, err := service.NewUserSearchClient("", "", "", "")
	require.NoError(t, err)
	users, err := userClient.SearchUsers("alice", 10)
	require.NoError(t, err)
	userClient.Close()

	require.NotNil(t, service.ConnectionClient)
	assert.True(t, service.ConnectionClient.Connected)
	assert.True(t, service.ConnectionClient.Closed)
	require.NotNil(t, service.UserSearchClient)
	assert.Equal(t, "alice", service.UserSearchClient.LastQuery)
	assert.Equal(t, 10, service.UserSearchClient.LastLimit)
	assert.Empty(t, users)
	assert.True(t, service.UserSearchClient.Closed)
}

func TestADServicePropagatesConfiguredErrors(t *testing.T) {
	service := &ADService{
		ConnectionClient: &ADConnectionClient{ConnectErr: errors.New("invalid credentials")},
		UserClientErr:    errors.New("directory unavailable"),
	}

	connectionClient := service.NewConnectionClient("", "", "", "")
	assert.EqualError(t, connectionClient.Connect(), "invalid credentials")

	userClient, err := service.NewUserSearchClient("", "", "", "")
	assert.Nil(t, userClient)
	assert.EqualError(t, err, "directory unavailable")
	assert.NotNil(t, service.ConnectionClient)
	assert.Nil(t, service.UserSearchClient)
}

func TestADUserSearchClientReturnsUsers(t *testing.T) {
	client := &ADUserSearchClient{
		Users: []ad.User{{Username: "alice"}, {Username: "bob"}},
	}

	users, err := client.SearchUsers("team", 25)

	require.NoError(t, err)
	assert.Equal(t, []ad.User{{Username: "alice"}, {Username: "bob"}}, users)
	assert.Equal(t, "team", client.LastQuery)
	assert.Equal(t, 25, client.LastLimit)

	client.Close()
	assert.True(t, client.Closed)
}
