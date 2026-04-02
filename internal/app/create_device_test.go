package app

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"vpn-backend/internal/domain"
)

func TestCreateDeviceExecuteHappyPath(t *testing.T) {
	callLog := make([]string, 0)

	userRepository := &fakeUserRepository{
		callLog: &callLog,
		user: &domain.User{
			ID:     42,
			Status: domain.UserStatusActive,
		},
	}
	deviceRepository := &fakeDeviceRepository{
		callLog:           &callLog,
		getByPublicKeyErr: domain.ErrNotFound,
		createResult: &domain.Device{
			ID:                  100,
			UserID:              42,
			Name:                "Dad Phone",
			PublicKey:           "public-key",
			EncryptedPrivateKey: "encrypted-private-key",
			AssignedIP:          "10.67.0.2",
			Status:              domain.DeviceStatusActive,
		},
	}
	transport := &fakeVPNTransport{callLog: &callLog}
	keyGenerator := &fakeKeyGenerator{
		callLog: &callLog,
		keyPair: &domain.KeyPair{
			PublicKey:  "public-key",
			PrivateKey: "private-key",
		},
	}
	privateKeyCipher := &fakePrivateKeyCipher{
		callLog:       &callLog,
		encryptResult: "encrypted-private-key",
	}
	ipAllocator := &fakeIPAllocator{
		callLog: &callLog,
		ip:      "10.67.0.2",
	}
	clientConfigBuilder := &fakeClientConfigBuilder{
		callLog: &callLog,
		result:  "[Interface]\nPrivateKey = private-key\n",
	}

	useCase := NewCreateDeviceUseCase(
		userRepository,
		deviceRepository,
		nil,
		transport,
		keyGenerator,
		privateKeyCipher,
		ipAllocator,
		clientConfigBuilder,
	)

	result, err := useCase.Execute(context.Background(), CreateDeviceInput{
		UserID: 42,
		Name:   "Dad Phone",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result == nil {
		t.Fatal("Execute() result = nil, want non-nil")
	}

	if result.Device == nil {
		t.Fatal("Execute() result.Device = nil, want non-nil")
	}

	if result.Device.EncryptedPrivateKey != "encrypted-private-key" {
		t.Fatalf("Device.EncryptedPrivateKey = %q, want %q", result.Device.EncryptedPrivateKey, "encrypted-private-key")
	}

	if result.Device.AssignedIP != "10.67.0.2" {
		t.Fatalf("Device.AssignedIP = %q, want %q", result.Device.AssignedIP, "10.67.0.2")
	}

	if result.ClientConfig != "[Interface]\nPrivateKey = private-key\n" {
		t.Fatalf("ClientConfig = %q, want expected config", result.ClientConfig)
	}

	if got := transport.createPeerInput; got.PublicKey != "public-key" || got.AssignedIP != "10.67.0.2" {
		t.Fatalf("CreatePeer input = %#v, want public-key/10.67.0.2", got)
	}

	if transport.removePeerCalls != 0 {
		t.Fatalf("RemovePeer calls = %d, want %d", transport.removePeerCalls, 0)
	}

	if got := clientConfigBuilder.input; got.DeviceName != "Dad Phone" || got.ClientPrivateKey != "private-key" || got.ClientAddress != "10.67.0.2" {
		t.Fatalf("Build input = %#v, want expected values", got)
	}

	if got := privateKeyCipher.encryptPlaintext; got != "private-key" {
		t.Fatalf("Encrypt plaintext = %q, want %q", got, "private-key")
	}

	if got := deviceRepository.createdDevice; got == nil {
		t.Fatal("created device = nil, want non-nil")
	} else {
		if got.PublicKey != "public-key" || got.EncryptedPrivateKey != "encrypted-private-key" || got.AssignedIP != "10.67.0.2" {
			t.Fatalf("created device = %#v, want expected values", got)
		}
	}

	wantCalls := []string{
		"user.get_by_id",
		"key.generate",
		"device.get_by_public_key",
		"ip.allocate_next",
		"transport.create_peer",
		"config_builder.build",
		"cipher.encrypt",
		"device.create",
	}
	if !reflect.DeepEqual(callLog, wantCalls) {
		t.Fatalf("call log = %#v, want %#v", callLog, wantCalls)
	}
}

func TestCreateDeviceExecuteCompensatesWhenConfigBuilderFails(t *testing.T) {
	callLog := make([]string, 0)
	builderErr := errors.New("build config failed")

	userRepository := &fakeUserRepository{
		callLog: &callLog,
		user: &domain.User{
			ID:     42,
			Status: domain.UserStatusActive,
		},
	}
	deviceRepository := &fakeDeviceRepository{
		callLog:           &callLog,
		getByPublicKeyErr: domain.ErrNotFound,
	}
	transport := &fakeVPNTransport{callLog: &callLog}
	keyGenerator := &fakeKeyGenerator{
		callLog: &callLog,
		keyPair: &domain.KeyPair{
			PublicKey:  "public-key",
			PrivateKey: "private-key",
		},
	}
	privateKeyCipher := &fakePrivateKeyCipher{callLog: &callLog}
	ipAllocator := &fakeIPAllocator{
		callLog: &callLog,
		ip:      "10.67.0.2",
	}
	clientConfigBuilder := &fakeClientConfigBuilder{
		callLog: &callLog,
		err:     builderErr,
	}

	useCase := NewCreateDeviceUseCase(
		userRepository,
		deviceRepository,
		nil,
		transport,
		keyGenerator,
		privateKeyCipher,
		ipAllocator,
		clientConfigBuilder,
	)

	result, err := useCase.Execute(context.Background(), CreateDeviceInput{
		UserID: 42,
		Name:   "Dad Phone",
	})
	if !errors.Is(err, builderErr) {
		t.Fatalf("Execute() error = %v, want %v", err, builderErr)
	}

	if result != nil {
		t.Fatalf("Execute() result = %#v, want nil", result)
	}

	if transport.createPeerCalls != 1 {
		t.Fatalf("CreatePeer calls = %d, want %d", transport.createPeerCalls, 1)
	}

	if transport.removePeerCalls != 1 {
		t.Fatalf("RemovePeer calls = %d, want %d", transport.removePeerCalls, 1)
	}

	if got := transport.removePeerInput; got.PublicKey != "public-key" || got.AssignedIP != "10.67.0.2" {
		t.Fatalf("RemovePeer input = %#v, want public-key/10.67.0.2", got)
	}

	if privateKeyCipher.encryptCalls != 0 {
		t.Fatalf("Encrypt calls = %d, want %d", privateKeyCipher.encryptCalls, 0)
	}

	if deviceRepository.createCalls != 0 {
		t.Fatalf("device create calls = %d, want %d", deviceRepository.createCalls, 0)
	}

	wantCalls := []string{
		"user.get_by_id",
		"key.generate",
		"device.get_by_public_key",
		"ip.allocate_next",
		"transport.create_peer",
		"config_builder.build",
		"transport.remove_peer",
	}
	if !reflect.DeepEqual(callLog, wantCalls) {
		t.Fatalf("call log = %#v, want %#v", callLog, wantCalls)
	}
}

type fakeUserRepository struct {
	callLog *[]string
	user    *domain.User
	err     error
}

func (f *fakeUserRepository) GetByID(context.Context, int64) (*domain.User, error) {
	*f.callLog = append(*f.callLog, "user.get_by_id")
	return f.user, f.err
}

func (f *fakeUserRepository) GetByTelegramID(context.Context, int64) (*domain.User, error) {
	return nil, nil
}

func (f *fakeUserRepository) Create(context.Context, domain.User) (*domain.User, error) {
	return nil, nil
}

func (f *fakeUserRepository) Update(context.Context, domain.User) (*domain.User, error) {
	return nil, nil
}

type fakeDeviceRepository struct {
	callLog           *[]string
	getByPublicKeyErr error
	createErr         error
	createResult      *domain.Device
	createdDevice     *domain.Device
	createCalls       int
}

func (f *fakeDeviceRepository) GetByID(context.Context, int64) (*domain.Device, error) {
	return nil, nil
}

func (f *fakeDeviceRepository) GetByPublicKey(context.Context, string) (*domain.Device, error) {
	*f.callLog = append(*f.callLog, "device.get_by_public_key")
	return nil, f.getByPublicKeyErr
}

func (f *fakeDeviceRepository) ListByUserID(context.Context, int64) ([]domain.Device, error) {
	return nil, nil
}

func (f *fakeDeviceRepository) Create(_ context.Context, device domain.Device) (*domain.Device, error) {
	*f.callLog = append(*f.callLog, "device.create")
	f.createCalls++
	deviceCopy := device
	f.createdDevice = &deviceCopy
	if f.createErr != nil {
		return nil, f.createErr
	}
	return f.createResult, nil
}

func (f *fakeDeviceRepository) Update(context.Context, domain.Device) (*domain.Device, error) {
	return nil, nil
}

type fakeVPNTransport struct {
	callLog         *[]string
	createPeerCalls int
	removePeerCalls int
	createPeerInput domain.CreatePeerInput
	removePeerInput domain.RemovePeerInput
	createPeerErr   error
	removePeerErr   error
}

func (f *fakeVPNTransport) CreatePeer(_ context.Context, input domain.CreatePeerInput) (*domain.Peer, error) {
	*f.callLog = append(*f.callLog, "transport.create_peer")
	f.createPeerCalls++
	f.createPeerInput = input
	if f.createPeerErr != nil {
		return nil, f.createPeerErr
	}
	return &domain.Peer{PublicKey: input.PublicKey, AssignedIP: input.AssignedIP}, nil
}

func (f *fakeVPNTransport) RemovePeer(_ context.Context, input domain.RemovePeerInput) error {
	*f.callLog = append(*f.callLog, "transport.remove_peer")
	f.removePeerCalls++
	f.removePeerInput = input
	return f.removePeerErr
}

func (f *fakeVPNTransport) GetPeerStatus(context.Context, domain.GetPeerStatusInput) (*domain.PeerStatus, error) {
	return nil, nil
}

func (f *fakeVPNTransport) Reconcile(context.Context, domain.ReconcileInput) (*domain.ReconcileResult, error) {
	return nil, nil
}

type fakeKeyGenerator struct {
	callLog *[]string
	keyPair *domain.KeyPair
	err     error
}

func (f *fakeKeyGenerator) Generate() (*domain.KeyPair, error) {
	*f.callLog = append(*f.callLog, "key.generate")
	return f.keyPair, f.err
}

type fakePrivateKeyCipher struct {
	callLog          *[]string
	encryptPlaintext string
	encryptResult    string
	encryptErr       error
	encryptCalls     int
}

func (f *fakePrivateKeyCipher) Encrypt(_ context.Context, plaintext string) (string, error) {
	*f.callLog = append(*f.callLog, "cipher.encrypt")
	f.encryptCalls++
	f.encryptPlaintext = plaintext
	return f.encryptResult, f.encryptErr
}

func (f *fakePrivateKeyCipher) Decrypt(context.Context, string) (string, error) {
	return "", nil
}

type fakeIPAllocator struct {
	callLog *[]string
	ip      string
	err     error
}

func (f *fakeIPAllocator) AllocateNext(context.Context) (string, error) {
	*f.callLog = append(*f.callLog, "ip.allocate_next")
	return f.ip, f.err
}

type fakeClientConfigBuilder struct {
	callLog *[]string
	input   domain.BuildClientConfigInput
	result  string
	err     error
}

func (f *fakeClientConfigBuilder) Build(_ context.Context, input domain.BuildClientConfigInput) (string, error) {
	*f.callLog = append(*f.callLog, "config_builder.build")
	f.input = input
	return f.result, f.err
}
