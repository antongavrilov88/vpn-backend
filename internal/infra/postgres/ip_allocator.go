package postgres

import (
	"context"
	"fmt"
	"net/netip"

	"github.com/jackc/pgx/v5/pgxpool"

	"vpn-backend/internal/domain"
)

const (
	defaultServerTunnelAddress = "10.67.0.1/24"
)

type IPAllocator struct {
	db               *pgxpool.Pool
	clientSubnet     netip.Prefix
	serverReservedIP netip.Addr
}

var _ domain.IPAllocator = (*IPAllocator)(nil)

type IPAllocatorConfig struct {
	ServerTunnelAddress string
}

func NewIPAllocator(db *pgxpool.Pool, cfg IPAllocatorConfig) (*IPAllocator, error) {
	clientSubnet, serverReservedIP, err := allocatorNetworkConfig(cfg.ServerTunnelAddress)
	if err != nil {
		return nil, err
	}

	return &IPAllocator{
		db:               db,
		clientSubnet:     clientSubnet,
		serverReservedIP: serverReservedIP,
	}, nil
}

func (a *IPAllocator) AllocateNext(ctx context.Context) (string, error) {
	const query = `
SELECT host(assigned_ip)
FROM devices
ORDER BY assigned_ip ASC
`

	rows, err := a.db.Query(ctx, query)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	assignedIPs := make([]string, 0)
	for rows.Next() {
		var ip string

		if err := rows.Scan(&ip); err != nil {
			return "", err
		}

		assignedIPs = append(assignedIPs, ip)
	}

	if err := rows.Err(); err != nil {
		return "", err
	}

	usedIPs, err := buildUsedIPSet(assignedIPs)
	if err != nil {
		return "", err
	}

	subnet := a.clientSubnet
	networkIP := subnet.Addr()
	broadcastIP := lastAddress(subnet)
	reservedServerIP := a.serverReservedIP

	usedIPs[networkIP] = struct{}{}
	usedIPs[broadcastIP] = struct{}{}
	usedIPs[reservedServerIP] = struct{}{}

	for candidate := networkIP.Next(); subnet.Contains(candidate); candidate = candidate.Next() {
		if candidate == broadcastIP {
			break
		}

		if _, exists := usedIPs[candidate]; exists {
			continue
		}

		return candidate.String(), nil
	}

	return "", domain.ErrIPPoolExhausted
}

func allocatorNetworkConfig(serverTunnelAddress string) (netip.Prefix, netip.Addr, error) {
	if serverTunnelAddress == "" {
		serverTunnelAddress = defaultServerTunnelAddress
	}

	serverPrefix, err := netip.ParsePrefix(serverTunnelAddress)
	if err != nil {
		return netip.Prefix{}, netip.Addr{}, err
	}

	serverAddr := serverPrefix.Addr()
	if !serverAddr.Is4() {
		return netip.Prefix{}, netip.Addr{}, fmt.Errorf("server tunnel address must be IPv4")
	}

	return serverPrefix.Masked(), serverAddr, nil
}

func buildUsedIPSet(assignedIPs []string) (map[netip.Addr]struct{}, error) {
	usedIPs := make(map[netip.Addr]struct{}, len(assignedIPs))

	for _, rawIP := range assignedIPs {
		addr, err := netip.ParseAddr(rawIP)
		if err != nil {
			prefix, prefixErr := netip.ParsePrefix(rawIP)
			if prefixErr != nil {
				return nil, err
			}

			addr = prefix.Addr()
		}

		usedIPs[addr] = struct{}{}
	}

	return usedIPs, nil
}

func lastAddress(prefix netip.Prefix) netip.Addr {
	addr := prefix.Addr().As4()
	bits := prefix.Bits()
	hostMask := uint32((1 << (32 - bits)) - 1)
	base := uint32(addr[0])<<24 | uint32(addr[1])<<16 | uint32(addr[2])<<8 | uint32(addr[3])
	last := base | hostMask

	return netip.AddrFrom4([4]byte{
		byte(last >> 24),
		byte(last >> 16),
		byte(last >> 8),
		byte(last),
	})
}
