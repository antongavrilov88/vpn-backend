package domain

import (
	"context"
	"time"
)

type VPNTransport interface {
	CreatePeer(ctx context.Context, input CreatePeerInput) (*Peer, error)
	RemovePeer(ctx context.Context, input RemovePeerInput) error
	GetPeerStatus(ctx context.Context, input GetPeerStatusInput) (*PeerStatus, error)
	Reconcile(ctx context.Context, input ReconcileInput) (*ReconcileResult, error)
}

type CreatePeerInput struct {
	PublicKey  string
	AssignedIP string
}

type RemovePeerInput struct {
	PublicKey  string
	AssignedIP string
}

type GetPeerStatusInput struct {
	PublicKey  string
	AssignedIP string
}

type ReconcileInput struct {
	Peers []Peer
}

type Peer struct {
	PublicKey  string
	AssignedIP string
}

type PeerStatus struct {
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
