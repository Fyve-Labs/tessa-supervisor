package tunnel

import (
	"context"

	"github.com/jpillora/chisel/client"
)

type Config struct {
	Server  string
	Remotes []string
	TLS     chclient.TLSConfig
}

func Start(ctx context.Context, c *Config) (*chclient.Client, error) {
	client, err := chclient.NewClient(&chclient.Config{
		Server:  c.Server,
		Remotes: c.Remotes,
		TLS:     c.TLS,
	})

	if err != nil {
		return nil, err
	}

	client.Debug = true

	if err = client.Start(ctx); err != nil {
		return nil, err
	}

	//if err = client.Wait(); err != nil {
	//	return nil, err
	//}

	return client, nil
}
