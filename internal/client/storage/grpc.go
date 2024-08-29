package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/GintGld/fizteh-radio/internal/lib/logger/sl"

	ssov1 "github.com/GintGld/fizteh-radio-proto/gen/go/storage"
	grpclog "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	grpcretry "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/retry"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
)

// TODO wrap streaming into io.Reader, io.Writer correctly
// TODO manage buffer len.

const (
	bufferLen = 1024 * 32
)

type Client struct {
	log *slog.Logger
	api ssov1.FileServiceClient
}

func New(
	ctx context.Context,
	log *slog.Logger,
	addr string,
	timeout time.Duration,
	retriesCount int,
) (*Client, error) {
	const op = "Client.New"

	retryOpts := []grpcretry.CallOption{
		grpcretry.WithCodes(codes.NotFound, codes.Aborted, codes.DeadlineExceeded),
		grpcretry.WithMax(uint(retriesCount)),
		grpcretry.WithPerRetryTimeout(timeout),
	}

	logOpts := []grpclog.Option{
		grpclog.WithLogOnEvents(grpclog.PayloadReceived, grpclog.PayloadSent),
	}

	// TODO options for secure
	cc, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(
			grpclog.UnaryClientInterceptor(InterceptorLogger(log), logOpts...),
			grpcretry.UnaryClientInterceptor(retryOpts...),
		),
	)
	if err != nil {
		log.Error("failed to create dial connect", sl.Err(err))
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return &Client{
		log: log,
		api: ssov1.NewFileServiceClient(cc),
	}, nil
}

// Upload sends data to gRPC.
func (c *Client) Upload(ctx context.Context, r io.Reader) (int, error) {
	const op = "Client.Upload"

	// Open upload stream.
	stream, err := c.api.Upload(ctx)
	if err != nil {
		fmt.Println("11111111111111")
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	// Send file.
	buff := make([]byte, bufferLen)
	for {
		n, err := r.Read(buff)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return 0, fmt.Errorf("%s: %w", op, err)
		}
		stream.Send(&ssov1.UploadRequest{Chunk: buff[:n]})
	}

	// Close stream
	resp, err := stream.CloseAndRecv()
	if err != nil {
		fmt.Println("ffuuuuuuuck", err)
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	return int(resp.GetFileId()), nil
}

// Download recieves data from gRPC.
func (c *Client) Download(ctx context.Context, id int, dst string) error {
	const op = "Client.Download"

	// Open download stream.
	stream, err := c.api.Download(ctx, &ssov1.DownloadRequest{FileId: int32(id)})
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	// Open file to write.
	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()

	// Copy data to a file.
	for {
		resp, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return fmt.Errorf("%s: %w", op, err)
		}

		if _, err := destination.Write(resp.GetChunk()); err != nil {
			return fmt.Errorf("%s: %w", op, err)
		}
	}

	// Close download stream.
	if err := stream.CloseSend(); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

// Delete deletes files by its id.
func (c *Client) Delete(ctx context.Context, id int) error {
	const op = "Client.Delete"

	resp, err := c.api.Delete(ctx, &ssov1.DeleteRequest{FileId: int32(id)})
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	if !resp.GetSuccess() {
		return errors.New("file not deleted")
	}

	return nil
}

func InterceptorLogger(l *slog.Logger) grpclog.Logger {
	return grpclog.LoggerFunc(func(ctx context.Context, lvl grpclog.Level, msg string, fields ...any) {
		l.Log(ctx, slog.Level(lvl), msg, fields...)
	})
}
