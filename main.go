package main

import (
	"context"
	"fmt"
	"google.golang.org/grpc/reflection"
	"net"
	"os"
	"os/signal"
	"syscall"

	pb "github.com/withchili/vkintern/pb"
	"github.com/withchili/vkintern/subpub"
	"google.golang.org/grpc"
)

func main() {
	cfg, err := LoadConfig("config.yaml")
	if err != nil {
		panic(err)
	}
	log := NewLogger().Sugar()
	log.Infow("config loaded", "port", cfg.GRPCPort)

	broker := subpub.NewSubPub()

	grpcServer := grpc.NewServer()
	pb.RegisterPubSubServer(grpcServer, NewServer(broker, log.Desugar()))
	reflection.Register(grpcServer)

	lis, err := net.Listen("tcp", ":"+fmt.Sprint(cfg.GRPCPort))
	if err != nil {
		log.Fatalw("cannot listen", "err", err)
	}
	go func() {
		log.Infow("gRPC started", "addr", lis.Addr().String())
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalw("grpc serve failed", "err", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Infow("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	go grpcServer.GracefulStop()
	if err := broker.Close(ctx); err != nil {
		log.Warnw("broker close timeout", "err", err)
	}

	log.Infow("bye")
}
