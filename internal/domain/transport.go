package domain

import (
	"context"
	"time"
)

type VPNTransport interface {
	CreatePeer(ctx context.Context, input CreatePeerInput) (*Peer, error)
	RemovePeer(ctx context.Context, input RemovePeerInput) error
	BuildClientConfig(ctx context.Context, input BuildClientConfigInput) (string, error)
	GetPeerStatus(ctx context.Context, input GetPeerStatusInput) (*PeerStatus, error)
	Reconcile(ctx context.Context, input ReconcileInput) (*ReconcileResult, error)
}

type CreatePeerInput struct {
	DeviceID   int64
	PublicKey  string
	AssignedIP string
}

type RemovePeerInput struct {
	DeviceID   int64
	PublicKey  string
	AssignedIP string
}

type BuildClientConfigInput struct {
	DeviceID            int64
	ServerPublicKey     string
	ClientPrivateKey    string
	ClientAddress       string
	ServerEndpoint      string
	ServerAllowedIPs    []string
	DNS                 []string
	PresharedKey        string
	PersistentKeepalive *int32
}

type GetPeerStatusInput struct {
	DeviceID   int64
	PublicKey  string
	AssignedIP string
}

type ReconcileInput struct {
	Peers []Peer
}

type Peer struct {
	DeviceID   int64
	PublicKey  string
	AssignedIP string
}

type PeerStatus struct {
	DeviceID        int64
	PublicKey       string
	AssignedIP      string
	IsPresent       bool
	IsConnected     bool
	BytesReceived   int64
	BytesSent       int64
	LastHandshakeAt *time.Time
}

type ReconcileResult struct {
	Created []Peer
	Removed []Peer
	Updated []Peer
}
