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

// Listen creates a TCP listener bound to host:port, selecting
// a default host IP if none is provided.
//
// Parameters:
//   - mode: "private" or "public", used to decide IP selection.
//   - host: optional IP address; if empty, pickIP(mode) is used.
//   - port: TCP port; if 0, a random free port is chosen.
func Listen(mode, host string, port int) (net.Listener, error) {
	if host == "" {
		ip, err := pickIP(mode)
		if err != nil {
			return nil, err
		}
		host = ip.String()
	} else {
		ip := net.ParseIP(host)
		if ip == nil {
			return nil, fmt.Errorf("invalid IP address: %s", host)
		}
		if mode == "private" && !isPrivateIP(ip) {
			return nil, fmt.Errorf("host %s is not private but mode=private", host)
		}
		if mode == "public" && isPrivateIP(ip) {
			return nil, fmt.Errorf("host %s is private but mode=public", host)
		}
	}
	addr := fmt.Sprintf("%s:%d", host, port)
	return net.Listen("tcp", addr)
}
