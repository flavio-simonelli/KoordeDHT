package server

import (
	"fmt"
	"net"
)

// pickIP selects a suitable IPv4 address from the local interfaces
// according to the given mode ("private" or "public").
//
// Rules:
//   - Only considers interfaces that are up and not loopback.
//   - Only considers IPv4 addresses (IPv6 is skipped).
//   - If mode == "private", returns the first private address found.
//   - If mode == "public", returns the first non-private address found.
//
// Returns an error if no suitable address is found.
func pickIP(mode string) (net.IP, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, iface := range ifaces {
		// Skip interfaces that are down or loopback
		if (iface.Flags&net.FlagUp) == 0 || (iface.Flags&net.FlagLoopback) != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			ip = ip.To4()
			if ip == nil {
				continue
			}

			if mode == "private" && isPrivateIP(ip) {
				return ip, nil
			}
			if mode == "public" && !isPrivateIP(ip) {
				return ip, nil
			}
		}
	}
	return nil, fmt.Errorf("no suitable %s interface found", mode)
}

// isPrivateIP checks whether the given IPv4 address
// belongs to one of the RFC1918 private address ranges.
func isPrivateIP(ip net.IP) bool {
	privateBlocks := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
	}
	for _, block := range privateBlocks {
		_, cidr, _ := net.ParseCIDR(block)
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

// Listen creates a TCP listener bound to bind:port,
// and returns the advertised address (host:port) to share with peers.
//
// Parameters:
//   - mode: "private" | "public" (usato solo se host è vuoto)
//   - bind: indirizzo su cui fare Listen (es. "0.0.0.0")
//   - host: indirizzo/hostname pubblicizzato (se vuoto → calcolato con mode)
//   - port: porta TCP
func Listen(mode, bind, host string, port int) (net.Listener, string, error) {
	// 1. Bind address
	if bind == "" {
		bind = "0.0.0.0"
	}
	bindAddr := fmt.Sprintf("%s:%d", bind, port)

	ln, err := net.Listen("tcp", bindAddr)
	if err != nil {
		return nil, "", err
	}

	actualPort := ln.Addr().(*net.TCPAddr).Port

	// 2. Advertised host
	if host == "" {
		ip, err := pickIP(mode)
		if err != nil {
			return nil, "", err
		}
		host = ip.String()
	} else {
		// se host è un IP, validato
		ip := net.ParseIP(host)
		if ip != nil {
			if mode == "private" && !isPrivateIP(ip) {
				return nil, "", fmt.Errorf("host %s is not private but mode=private", host)
			}
			if mode == "public" && isPrivateIP(ip) {
				return nil, "", fmt.Errorf("host %s is private but mode=public", host)
			}
		}
		// se host NON è un IP (es. "node7"), lo accettiamo come hostname advertised
	}

	advertised := fmt.Sprintf("%s:%d", host, actualPort)
	return ln, advertised, nil
}
