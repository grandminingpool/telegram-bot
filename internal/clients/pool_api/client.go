package poolAPIClient

import (
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func NewClient(address, certsPath, caCertFile string) (*grpc.ClientConn, error) {
	creds, err := credentials.NewClientTLSFromFile(fmt.Sprintf("%s/%s", certsPath, caCertFile), "")
	if err != nil {
		return nil, fmt.Errorf("failed to load pool api client certificate: %w", err)
	}

	client, err := grpc.NewClient(address, grpc.WithTransportCredentials(creds))
	if err != nil {
		return nil, fmt.Errorf("failed to create pool api client: %w", err)
	}

	return client, nil
}
