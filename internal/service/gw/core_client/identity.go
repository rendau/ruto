package core_client

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type identity struct {
	GatewayID string
	PodUID    string
	PodName   string
	NodeName  string
	HostName  string
}

func newIdentity() *identity {
	hostName, _ := os.Hostname()
	hostName = strings.TrimSpace(hostName)

	nodeName := strings.TrimSpace(os.Getenv("K8S_NODE_NAME"))
	podUID := strings.TrimSpace(os.Getenv("K8S_POD_UID"))
	podName := strings.TrimSpace(os.Getenv("K8S_POD_NAME"))

	gatewayID := buildGatewayID(nodeName, podUID, podName, hostName)

	return &identity{
		GatewayID: gatewayID,
		PodUID:    podUID,
		PodName:   podName,
		NodeName:  nodeName,
		HostName:  hostName,
	}
}

func buildGatewayID(nodeName, podUID, podName, hostName string) string {
	if podUID != "" {
		return "k8s-pod:" + podUID
	}
	if nodeName != "" && podName != "" {
		return "k8s-node:" + nodeName + ":pod:" + podName
	}
	if hostName != "" {
		return fmt.Sprintf("%s(%s)", hostName, strconv.Itoa(os.Getpid()))
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
