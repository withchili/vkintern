package main

import (
	"context"
	"sync/atomic"

	pb "github.com/withchili/vkintern/pb"
	"github.com/withchili/vkintern/subpub"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	emptypb "google.golang.org/protobuf/types/known/emptypb"

	"go.uber.org/zap"
)

type Server struct {
	pb.UnimplementedPubSubServer
	broker    subpub.SubPub
	log       *zap.Logger
	nextSubID int64
}

func NewServer(b subpub.SubPub, log *zap.Logger) *Server {
	return &Server{broker: b, log: log}
}

func (s *Server) Publish(ctx context.Context, req *pb.PublishRequest) (*emptypb.Empty, error) {
	if req.GetKey() == "" {
		return nil, status.Error(codes.InvalidArgument, "key is empty")
	}
	var addr string
	if p, ok := peer.FromContext(ctx); ok && p.Addr != nil {
		addr = p.Addr.String()
	}
	s.log.Info("publish",
		zap.String("addr", addr),
		zap.String("key", req.GetKey()),
		zap.String("data", req.GetData()),
	)

	ev := &pb.Event{Data: req.GetData()}
	if err := s.broker.Publish(req.GetKey(), ev); err != nil {
		s.log.Warn("publish failed", zap.String("key", req.GetKey()), zap.Error(err))
		return nil, status.Error(codes.Unavailable, err.Error())
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) Subscribe(req *pb.SubscribeRequest, stream pb.PubSub_SubscribeServer) error {
	if req.GetKey() == "" {
		return status.Error(codes.InvalidArgument, "key is empty")
	}
	var addr string
	if p, ok := peer.FromContext(stream.Context()); ok && p.Addr != nil {
		addr = p.Addr.String()
	}
	id := atomic.AddInt64(&s.nextSubID, 1)
	s.log.Info("new subscription",
		zap.Int64("id", id),
		zap.String("addr", addr),
		zap.String("key", req.GetKey()),
	)

	sub, err := s.broker.Subscribe(req.GetKey(), func(msg interface{}) {
		if ev, ok := msg.(*pb.Event); ok {
			s.log.Info("deliver",
				zap.Int64("id", id),
				zap.String("key", req.GetKey()),
				zap.String("data", ev.GetData()),
			)
			_ = stream.Send(ev)
		}
	})
	if err != nil {
		return status.Error(codes.Unavailable, err.Error())
	}
	defer func() {
		sub.Unsubscribe()
		s.log.Info("subscription closed",
			zap.Int64("id", id),
			zap.String("key", req.GetKey()),
		)
	}()

	<-stream.Context().Done()
	return nil
}
