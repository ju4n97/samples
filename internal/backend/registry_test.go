package backend

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// --- Mock types ---

type MockBackend struct {
	mock.Mock
	name string
}

func (m *MockBackend) Name() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockBackend) Infer(ctx context.Context, req *Request) (*Response, error) {
	args := m.Called(ctx, req)
	if resp, ok := args.Get(0).(*Response); ok {
		return resp, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockBackend) Close() error {
	args := m.Called()
	return args.Error(0)
}

type MockStreamingBackend struct {
	MockBackend
}

func (m *MockStreamingBackend) InferStream(ctx context.Context, req *Request) (<-chan StreamChunk, error) {
	args := m.Called(ctx, req)
	if ch, ok := args.Get(0).(<-chan StreamChunk); ok {
		return ch, args.Error(1)
	}
	return nil, args.Error(1)
}

// --- Tests ---

func TestRegistry_RegisterAndGet(t *testing.T) {
	reg := NewRegistry()
	mockBackend := new(MockBackend)
	mockBackend.On("Name").Return("test-backend")

	reg.Register(mockBackend)

	got, ok := reg.Get("test-backend")
	assert.True(t, ok)
	assert.Equal(t, mockBackend, got)

	// Ensure a missing backend returns false
	_, ok = reg.Get("missing")
	assert.False(t, ok)

	mockBackend.AssertExpectations(t)
}

func TestRegistry_GetStreaming(t *testing.T) {
	reg := NewRegistry()

	// Non-streaming backend
	mockBackend := new(MockBackend)
	mockBackend.On("Name").Return("basic")
	reg.Register(mockBackend)

	sb, ok := reg.GetStreaming("basic")
	assert.False(t, ok)
	assert.Nil(t, sb)

	// Streaming backend
	mockStream := new(MockStreamingBackend)
	mockStream.On("Name").Return("streamer")
	reg.Register(mockStream)

	sb, ok = reg.GetStreaming("streamer")
	assert.True(t, ok)
	assert.Equal(t, mockStream, sb)

	mockBackend.AssertExpectations(t)
	mockStream.AssertExpectations(t)
}

func TestRegistry_Close(t *testing.T) {
	reg := NewRegistry()

	b1 := new(MockBackend)
	b2 := new(MockBackend)
	b1.On("Name").Return("b1")
	b2.On("Name").Return("b2")

	// Normal close
	b1.On("Close").Return(nil).Once()
	b2.On("Close").Return(nil).Once()

	reg.Register(b1)
	reg.Register(b2)

	err := reg.Close()
	assert.NoError(t, err)

	b1.AssertExpectations(t)
	b2.AssertExpectations(t)
}

func TestRegistry_CloseErrorPropagation(t *testing.T) {
	reg := NewRegistry()

	b1 := new(MockBackend)
	b2 := new(MockBackend)

	b1.On("Name").Return("b1")
	b2.On("Name").Return("b2")

	b1.On("Close").Return(errors.New("close failed")).Once()
	b2.On("Close").Return(nil).Maybe()

	reg.Register(b1)
	reg.Register(b2)

	err := reg.Close()
	assert.EqualError(t, err, "close failed")

	b1.AssertExpectations(t)
	b2.AssertExpectations(t)
}
