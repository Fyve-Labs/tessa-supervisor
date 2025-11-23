package handler

import (
	"context"
	"encoding/json"
)

type Handler interface {
	Handle(ctx context.Context) error
	Stop() error
}

func JsonPayloadToConfig[T interface{}](payload interface{}) (*T, error) {
	var config T
	bytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	if err = json.Unmarshal(bytes, &config); err != nil {
		return nil, err
	}

	return &config, nil
}
