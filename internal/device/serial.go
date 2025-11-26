package client

import "os"

func Serial() string {
	if value := os.Getenv("DEVICE_NAME"); value != "" {
		return value
	}

	if bytes, err := os.ReadFile("/sys/firmware/devicetree/base/serial-number"); err == nil {
		return string(bytes)
	}

	// TODO: something went wrong
	return "unknow"
}
