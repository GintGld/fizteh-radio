package models

import (
	"errors"
	"fmt"
	"io"

	ssov1 "github.com/GintGld/fizteh-radio-proto/gen/go/storage"
	"google.golang.org/grpc"
)

type UploadStreamer struct {
	Stream grpc.ClientStreamingClient[ssov1.UploadRequest, ssov1.UploadResponse]
}

func (s *UploadStreamer) Write(p []byte) (int, error) {
	const op = "UploadStreamer.Write"

	if err := s.Stream.Send(&ssov1.UploadRequest{Chunk: p}); err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	return len(p), nil
}

type DownloadStreamer struct {
	Stream grpc.ServerStreamingClient[ssov1.DownloadResponse]
}

func (s *DownloadStreamer) Read(p []byte) (int, error) {
	const op = "DownloadStreamer.Read"

	recv, err := s.Stream.Recv()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return 0, io.EOF
		}
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	p = recv.GetChunk()

	return len(p), nil
}
