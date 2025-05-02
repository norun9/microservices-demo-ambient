package main

import (
	"context"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"log"
	"math/rand"
	"net"
	"os"
	"time"

	pb "github.com/norun9/microservices-demo-ambient/src/adservice-go/genproto/hipstershop"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

type adServiceServer struct {
	pb.UnimplementedAdServiceServer
}

var adsMap = map[string][]*pb.Ad{
	"clothing":    {{RedirectUrl: "/product/66VCHSJNUP", Text: "Tank top for sale. 20% off."}},
	"accessories": {{RedirectUrl: "/product/1YMWWN1N4O", Text: "Watch for sale. Buy one, get second kit for free"}},
	"footwear":    {{RedirectUrl: "/product/L9ECAV7KIM", Text: "Loafers for sale. Buy one, get second one for free"}},
	"hair":        {{RedirectUrl: "/product/2ZYFJ3GM2N", Text: "Hairdryer for sale. 50% off."}},
	"decor":       {{RedirectUrl: "/product/0PUK6V6EV0", Text: "Candle holder for sale. 30% off."}},
	"kitchen": {
		{RedirectUrl: "/product/9SIQT8TOJO", Text: "Bamboo glass jar for sale. 10% off."},
		{RedirectUrl: "/product/6E92ZMYYFZ", Text: "Mug for sale. Buy two, get third one for free"},
	},
}

func (s *adServiceServer) GetAds(ctx context.Context, req *pb.AdRequest) (*pb.AdResponse, error) {
	log.Printf("received ad request (context_words=%v)", req.ContextKeys)
	var ads []*pb.Ad
	for _, key := range req.ContextKeys {
		ads = append(ads, adsMap[key]...)
	}
	if len(ads) == 0 {
		ads = getRandomAds()
	}
	return &pb.AdResponse{Ads: ads}, nil
}

func getRandomAds() []*pb.Ad {
	var allAds []*pb.Ad
	for _, ads := range adsMap {
		allAds = append(allAds, ads...)
	}
	rand.Seed(time.Now().UnixNano())
	res := []*pb.Ad{}
	for i := 0; i < 2; i++ {
		res = append(res, allAds[rand.Intn(len(allAds))])
	}
	return res
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "9555"
	}
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)
	pb.RegisterAdServiceServer(grpcServer, &adServiceServer{})
	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	log.Printf("Ad Service started, listening on %s", port)
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
