package admocks

import "github.com/weibinliao/OpenAD/internal/ad"

type ADService struct {
	ConnectionClient *ADConnectionClient
	UserSearchClient *ADUserSearchClient
	UserClientErr    error
}

type ADConnectionClient struct {
	Connected  bool
	Closed     bool
	ConnectErr error
}

type ADUserSearchClient struct {
	Users     []ad.User
	SearchErr error
	Closed    bool
	LastQuery string
	LastLimit int
}

func (service *ADService) NewConnectionClient(_ string, _ string, _ string, _ string) interface {
	Connect() error
	Close()
} {
	if service.ConnectionClient == nil {
		service.ConnectionClient = &ADConnectionClient{}
	}

	return service.ConnectionClient
}

func (service *ADService) NewUserSearchClient(_ string, _ string, _ string, _ string) (interface {
	SearchUsers(query string, limit int) ([]ad.User, error)
	Close()
}, error) {
	if service.UserClientErr != nil {
		return nil, service.UserClientErr
	}

	if service.UserSearchClient == nil {
		service.UserSearchClient = &ADUserSearchClient{}
	}

	return service.UserSearchClient, nil
}

func (client *ADConnectionClient) Connect() error {
	if client.ConnectErr != nil {
		return client.ConnectErr
	}

	client.Connected = true
	return nil
}

func (client *ADConnectionClient) Close() {
	client.Closed = true
}

func (client *ADUserSearchClient) SearchUsers(query string, limit int) ([]ad.User, error) {
	client.LastQuery = query
	client.LastLimit = limit
	if client.SearchErr != nil {
		return nil, client.SearchErr
	}

	return append([]ad.User(nil), client.Users...), nil
}

func (client *ADUserSearchClient) Close() {
	client.Closed = true
}
