package postgres

import (
	"context"
	"net/netip"

	"github.com/jackc/pgx/v5/pgxpool"

	"vpn-backend/internal/domain"
)

const (
	clientSubnetCIDR = "10.67.0.0/24"
	serverReservedIP = "10.67.0.1"
)

type IPAllocator struct {
	db *pgxpool.Pool
}

var _ domain.IPAllocator = (*IPAllocator)(nil)

func NewIPAllocator(db *pgxpool.Pool) *IPAllocator {
	return &IPAllocator{db: db}
}

func (a *IPAllocator) AllocateNext(ctx context.Context) (string, error) {
	const query = `
SELECT assigned_ip::text
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

	subnet := netip.MustParsePrefix(clientSubnetCIDR)
	networkIP := subnet.Addr()
	broadcastIP := lastAddress(subnet)
	reservedServerIP := netip.MustParseAddr(serverReservedIP)

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

func buildUsedIPSet(assignedIPs []string) (map[netip.Addr]struct{}, error) {
	usedIPs := make(map[netip.Addr]struct{}, len(assignedIPs))

	for _, rawIP := range assignedIPs {
		addr, err := netip.ParseAddr(rawIP)
		if err != nil {
			return nil, err
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
