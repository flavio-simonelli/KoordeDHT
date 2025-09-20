package config

import (
	"fmt"
	"net"
)

// pickIP sceglie un indirizzo IP valido in base alla modalità
func pickIP(mode string) (net.IP, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, iface := range ifaces {
		// escludo interfacce spente o loopback
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

// isPrivateIP controlla se l’IP è in uno spazio privato
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

// Listen crea un net.Listener scegliendo host/porta corretti
func (cfg *NodeConfig) Listen() (net.Listener, error) {
	host := cfg.Server.Host
	if host == "" {
		ip, err := pickIP(cfg.DHT.Mode)
		if err != nil {
			return nil, err
		}
		host = ip.String()
	} else {
		// Se l'utente ha specificato un IP → validiamo rispetto alla mode
		ip := net.ParseIP(host)
		if ip == nil {
			return nil, fmt.Errorf("invalid IP address: %s", host)
		}
		if cfg.DHT.Mode == "private" && !isPrivateIP(ip) {
			return nil, fmt.Errorf("host %s is not private but mode=private", host)
		}
		if cfg.DHT.Mode == "public" && isPrivateIP(ip) {
			return nil, fmt.Errorf("host %s is private but mode=public", host)
		}
	}
	addr := fmt.Sprintf("%s:%d", host, cfg.Server.Port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	return lis, nil
}
