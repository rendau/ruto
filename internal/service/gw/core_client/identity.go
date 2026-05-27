package core_client

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

type identity struct {
	GatewayID string
	PodName   string
	HostName  string
}

func newIdentity() *identity {
	hostName, _ := os.Hostname()
	hostName = strings.TrimSpace(hostName)

	podName := strings.TrimSpace(os.Getenv("POD_NAME"))

	gatewayID := buildGatewayID(podName, hostName)

	return &identity{
		GatewayID: gatewayID,
		PodName:   podName,
		HostName:  hostName,
	}
}

func buildGatewayID(podName, hostName string) string {
	if podName != "" {
		return podName
	}
	if hostName != "" {
		return fmt.Sprintf("%s-%d", hostName, os.Getpid())
	}
	return "gw:" + randomHex(6)
}

func randomHex(byteCount int) string {
	if byteCount <= 0 {
		byteCount = 6
	}
	data := make([]byte, byteCount)
	if _, err := rand.Read(data); err != nil {
		return "random"
	}
	return hex.EncodeToString(data)
}
