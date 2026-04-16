package config

import (
	"strings"
)

type Endpoint struct {
	Id            string
	Method        string
	Path          string
	Backend       EndpointBackend
	JwtValidation EndpointJwtValidation
	IpValidation  EndpointIpValidation
}

type EndpointBackend struct {
	Path string
}

type EndpointJwtValidation struct {
	Enabled bool
	Roles   []string
}

type EndpointIpValidation struct {
	AllowedIps []string
}

func (m *Endpoint) Normalize() {
	m.Method = strings.ToUpper(strings.TrimSpace(m.Method))
	m.Path = strings.TrimPrefix(strings.TrimSpace(m.Path), "/")

	m.Backend.Normalize()
	m.JwtValidation.Normalize()
	m.IpValidation.Normalize()
}

func (m *EndpointBackend) Normalize() {
	m.Path = strings.TrimPrefix(strings.TrimSpace(m.Path), "/")
}

func (m *EndpointJwtValidation) Normalize() {
	finalRoles := make([]string, 0, len(m.Roles))
	for _, role := range m.Roles {
		role = strings.TrimSpace(role)
		if role == "" {
			continue
		}
		finalRoles = append(finalRoles, role)
	}
}

func (m *EndpointIpValidation) Normalize() {
	finalIps := make([]string, 0, len(m.AllowedIps))
	for _, ip := range m.AllowedIps {
		ip = strings.TrimSpace(ip)
		if ip == "" {
			continue
		}
		finalIps = append(finalIps, ip)
	}
}
