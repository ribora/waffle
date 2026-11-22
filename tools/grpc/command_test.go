package grpc

import (
	"context"
	"testing"
	"time"
)

type drainedService struct {
	healthService
	drained chan struct{}
}

func (s *drainedService) Drain() {
	close(s.drained)
}

type waitingServer struct {
	calls chan struct{}
}

func (s waitingServer) Serve(context.Context) error { return nil }

func (s waitingServer) Close() error {
	<-s.calls
	return nil
}

func TestCloseDrainsWhileTheServerWaitsForRunningCalls(t *testing.T) {
	service := &drainedService{drained: make(chan struct{})}
	command := NewGrpcCommand([]ServiceHandler{service}, nil, nil, nil, nil, 0, nil)
	command.connection = waitingServer{calls: service.drained}

	closed := make(chan error, 1)
	go func() { closed <- command.Close() }()

	select {
	case err := <-closed:
		if err != nil {
			t.Fatalf("close: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Close waits for calls only Drain ends")
	}
}

func TestCloseDrainsACommandThatNeverServed(t *testing.T) {
	service := &drainedService{drained: make(chan struct{})}
	command := NewGrpcCommand([]ServiceHandler{service, &healthService{}}, nil, nil, nil, nil, 0, nil)

	if err := command.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	select {
	case <-service.drained:
	default:
		t.Fatal("the handler was not drained")
	}
}
