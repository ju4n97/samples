package grpc

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"maps"

	"github.com/ekisa-team/syn4pse/internal/backend"
	"github.com/ekisa-team/syn4pse/internal/model"
	inferencev1 "github.com/ekisa-team/syn4pse/internal/pb/inference/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// InferenceServer implements inferencev1.InferenceServiceServer.
type InferenceServer struct {
	inferencev1.UnimplementedInferenceServiceServer
	backends *backend.Registry
	models   *model.Registry
}

// NewInferenceServer creates a new InferenceServer instance.
func NewInferenceServer(backends *backend.Registry, models *model.Registry) *InferenceServer {
	return &InferenceServer{
		backends: backends,
		models:   models,
	}
}

// Infer handles synchronous inference requests.
func (s *InferenceServer) Infer(ctx context.Context, req *inferencev1.InferenceRequest) (*inferencev1.InferenceResponse, error) {

	if err := validateInferenceRequest(req); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid request: %v", err)
	}

	b, ok := s.backends.Get(backend.BackendProvider(req.Provider))
	if !ok {
		return nil, status.Errorf(codes.NotFound, "backend not found: %s", req.Provider)
	}

	m, ok := s.models.Get(req.ModelId)
	if !ok {
		return nil, status.Errorf(codes.NotFound, "model not found: %s", req.ModelId)
	}

	parameters, err := parseParameters(req.Parameters)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to parse parameters: %v", err)
	}

	breq := &backend.Request{
		ModelPath:  m.Path,
		Input:      bytes.NewReader(req.Input),
		Parameters: parameters,
	}

	resp, err := b.Infer(ctx, breq)
	if err != nil {
		return nil, mapBackendError(err)
	}

	output, err := io.ReadAll(resp.Output)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to read output: %v", err)
	}

	return &inferencev1.InferenceResponse{
		Output:   output,
		Metadata: buildMetadata(resp.Metadata),
	}, nil
}

// InferStream handles streaming inference requests.
func (s *InferenceServer) InferStream(stream inferencev1.InferenceService_InferStreamServer) error {
	ctx := stream.Context()

	req, err := stream.Recv()
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "failed to receive initial request: %v", err)
	}

	if err := validateInferenceRequest(req); err != nil {
		return status.Errorf(codes.InvalidArgument, "invalid request: %v", err)
	}

	b, ok := s.backends.Get(backend.BackendProvider(req.Provider))
	if !ok {
		return status.Errorf(codes.NotFound, "backend not found: %s", req.Provider)
	}

	sb, ok := b.(backend.StreamingBackend)
	if !ok {
		return status.Errorf(codes.Unimplemented, "backend %s does not support streaming", req.Provider)
	}

	m, ok := s.models.Get(req.ModelId)
	if !ok {
		return status.Errorf(codes.NotFound, "model not found: %s", req.ModelId)
	}

	parameters, err := parseParameters(req.Parameters)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "failed to parse parameters: %v", err)
	}

	breq := &backend.Request{
		ModelPath:  m.Path,
		Input:      bytes.NewReader(req.Input),
		Parameters: parameters,
	}

	chunkChan, err := sb.InferStream(ctx, breq)
	if err != nil {
		return mapBackendError(err)
	}

	for chunk := range chunkChan {
		if chunk.Error != nil {
			if err := stream.Send(&inferencev1.StreamChunk{
				Data:  nil,
				Done:  true,
				Error: chunk.Error.Error(),
			}); err != nil {
				return status.Errorf(codes.Internal, "failed to send error chunk: %v", err)
			}
			return status.Errorf(codes.Unknown, "inference error: %v", chunk.Error)
		}

		if err := stream.Send(&inferencev1.StreamChunk{
			Data:  chunk.Data,
			Done:  chunk.Done,
			Error: "",
		}); err != nil {
			return status.Errorf(codes.Internal, "failed to send chunk: %v", err)
		}

		if chunk.Done {
			break
		}
	}

	return nil
}

// validateInferenceRequest validates the inference request.
func validateInferenceRequest(req *inferencev1.InferenceRequest) error {
	if req.Provider == "" {
		return fmt.Errorf("provider is required")
	}
	if req.ModelId == "" {
		return fmt.Errorf("model_id is required")
	}
	if len(req.Input) == 0 {
		return fmt.Errorf("input is required")
	}
	return nil
}

// buildMetadata converts backend metadata to protobuf metadata.
func buildMetadata(meta *backend.ResponseMetadata) *inferencev1.InferenceMetadata {
	if meta == nil {
		return nil
	}

	backendSpecific := make(map[string]string)
	if meta.BackendSpecific != nil {
		maps.Copy(backendSpecific, meta.BackendSpecific)
	}

	return &inferencev1.InferenceMetadata{
		Provider:        string(meta.Provider),
		Model:           meta.Model,
		Timestamp:       timestamppb.New(meta.Timestamp),
		OutputBytes:     meta.OutputBytes,
		BackendSpecific: backendSpecific,
	}
}

// parseParameters converts protobuf Value map to Go native types.
func parseParameters(params map[string]*structpb.Value) (map[string]any, error) {
	if params == nil {
		return make(map[string]any), nil
	}

	parameters := make(map[string]any, len(params))
	for k, v := range params {
		if v == nil {
			continue
		}

		switch kind := v.ProtoReflect().Get(v.ProtoReflect().Descriptor().Fields().ByName("kind")).Interface().(type) {
		case *structpb.Value_NullValue:
			parameters[k] = nil
		case *structpb.Value_NumberValue:
			parameters[k] = v.GetNumberValue()
		case *structpb.Value_StringValue:
			parameters[k] = v.GetStringValue()
		case *structpb.Value_BoolValue:
			parameters[k] = v.GetBoolValue()
		default:
			return nil, fmt.Errorf("unsupported parameter type for key %s: %T", k, kind)
		}
	}

	return parameters, nil
}

// mapBackendError converts backend errors to appropriate gRPC status codes.
func mapBackendError(err error) error {
	if err == nil {
		return nil
	}

	switch err {
	case backend.ErrBackendNotFound:
		return status.Error(codes.NotFound, err.Error())
	case model.ErrModelNotFound:
		return status.Error(codes.NotFound, err.Error())
	default:
		// Check if it's already a gRPC status error
		if _, ok := status.FromError(err); ok {
			return err
		}
		// Default to Unknown for other errors
		return status.Errorf(codes.Unknown, "backend error: %v", err)
	}
}
