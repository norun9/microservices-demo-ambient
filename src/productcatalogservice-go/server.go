// Copyright 2018 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	pb "github.com/norun9/opentelemetry-microservices-demo/src/productcatalogservice-go/genproto/hipstershop"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/sirupsen/logrus"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"google.golang.org/protobuf/encoding/protojson"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	cat          pb.ListProductsResponse
	catalogMutex *sync.Mutex
	log          *logrus.Logger
	extraLatency time.Duration
	tracer       trace.Tracer

	port = "3550"

	reloadCatalog bool
)

func init() {
	log = logrus.New()
	log.Formatter = &logrus.JSONFormatter{
		FieldMap: logrus.FieldMap{
			logrus.FieldKeyTime:  "timestamp",
			logrus.FieldKeyLevel: "severity",
			logrus.FieldKeyMsg:   "message",
		},
		TimestampFormat: time.RFC3339Nano,
	}
	log.Out = os.Stdout
	catalogMutex = &sync.Mutex{}
	err := readCatalogFile(&cat)
	if err != nil {
		log.Warnf("could not parse product catalog")
	}
}

func InitTracerProvider() *sdktrace.TracerProvider {
	ctx := context.Background()

	// OTLPエンドポイントの設定
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		endpoint = "dns:///otel-collector.observability.svc.cluster.local:4317"
	}

	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		log.Fatal(err)
	}

	// リソース情報の設定
	res, err := resource.New(ctx,
		resource.WithAttributes(
			attribute.String("service.name", "productcatalogservice"),
			attribute.String("service.version", "v1.0.0"),
		),
	)
	if err != nil {
		log.Fatal(err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	// トレーサーの初期化
	tracer = otel.Tracer("productcatalogservice")

	return tp
}

func main() {
	tp := InitTracerProvider()
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down tracer provider: %v", err)
		}
	}()

	flag.Parse()

	// set injected latency
	if s := os.Getenv("EXTRA_LATENCY"); s != "" {
		v, err := time.ParseDuration(s)
		if err != nil {
			log.Fatalf("failed to parse EXTRA_LATENCY (%s) as time.Duration: %+v", v, err)
		}
		extraLatency = v
		log.Infof("extra latency enabled (duration: %v)", extraLatency)
	} else {
		extraLatency = time.Duration(0)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGUSR1, syscall.SIGUSR2)
	go func() {
		for {
			sig := <-sigs
			log.Printf("Received signal: %s", sig)
			if sig == syscall.SIGUSR1 {
				reloadCatalog = true
				log.Infof("Enable catalog reloading")
			} else {
				reloadCatalog = false
				log.Infof("Disable catalog reloading")
			}
		}
	}()

	if os.Getenv("PORT") != "" {
		port = os.Getenv("PORT")
	}
	log.Infof("starting grpc server at :%s", port)
	run(port)
	select {}
}

func run(port string) string {
	l, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		log.Fatal(err)
	}

	var srv *grpc.Server = grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)

	svc := &productCatalog{}
	healthSvc := &HealthService{}

	pb.RegisterProductCatalogServiceServer(srv, svc)
	healthpb.RegisterHealthServer(srv, healthSvc)
	go srv.Serve(l)
	return l.Addr().String()
}

type productCatalog struct {
	pb.UnimplementedProductCatalogServiceServer
}

func readCatalogFile(catalog *pb.ListProductsResponse) error {
	catalogMutex.Lock()
	defer catalogMutex.Unlock()

	catalogJSON, err := ioutil.ReadFile("products.json")
	if err != nil {
		log.Fatalf("failed to open product catalog json file: %v", err)
		return err
	}

	if err := protojson.Unmarshal(catalogJSON, catalog); err != nil {
		log.Warnf("failed to parse the catalog JSON: %v", err)
		return err
	}

	log.Info("successfully parsed product catalog json")
	return nil
}

func parseCatalog() []*pb.Product {
	if reloadCatalog || len(cat.Products) == 0 {
		err := readCatalogFile(&cat)
		if err != nil {
			return []*pb.Product{}
		}
	}

	return cat.Products
}

func (p *productCatalog) ListProducts(ctx context.Context, req *pb.Empty) (*pb.ListProductsResponse, error) {
	_, span := tracer.Start(ctx, "ListProducts")
	defer span.End()

	span.SetAttributes(attribute.String("request.type", "list_all"))

	time.Sleep(extraLatency)

	products := parseCatalog()

	span.SetAttributes(
		attribute.Int("response.products.count", len(products)),
		attribute.String("extra_latency", extraLatency.String()),
	)

	return &pb.ListProductsResponse{Products: products}, nil
}

func (p *productCatalog) GetProduct(ctx context.Context, req *pb.GetProductRequest) (*pb.Product, error) {
	_, span := tracer.Start(ctx, "GetProduct")
	defer span.End()

	span.SetAttributes(
		attribute.String("request.product_id", req.Id),
		attribute.String("request.type", "get_single"),
	)

	time.Sleep(extraLatency)

	var found *pb.Product
	products := parseCatalog()

	for i := 0; i < len(products); i++ {
		if req.Id == products[i].Id {
			found = products[i]
			break
		}
	}

	if found == nil {
		span.SetAttributes(
			attribute.String("error", "product not found"),
			attribute.String("error.code", "NOT_FOUND"),
		)
		return nil, status.Errorf(codes.NotFound, "no product with ID %s", req.Id)
	}

	span.SetAttributes(
		attribute.String("response.product.name", found.Name),
		attribute.String("response.product.id", found.Id),
		attribute.String("extra_latency", extraLatency.String()),
	)

	return found, nil
}

func (p *productCatalog) SearchProducts(ctx context.Context, req *pb.SearchProductsRequest) (*pb.SearchProductsResponse, error) {
	_, span := tracer.Start(ctx, "SearchProducts")
	defer span.End()

	span.SetAttributes(
		attribute.String("request.query", req.Query),
		attribute.String("request.type", "search"),
	)

	time.Sleep(extraLatency)

	// Intepret query as a substring match in name or description.
	var ps []*pb.Product
	products := parseCatalog()

	for _, p := range products {
		if strings.Contains(strings.ToLower(p.Name), strings.ToLower(req.Query)) ||
			strings.Contains(strings.ToLower(p.Description), strings.ToLower(req.Query)) {
			ps = append(ps, p)
		}
	}

	span.SetAttributes(
		attribute.Int("response.results.count", len(ps)),
		attribute.String("extra_latency", extraLatency.String()),
	)

	return &pb.SearchProductsResponse{Results: ps}, nil
}

// Health Service implementation
type HealthService struct {
	healthpb.UnimplementedHealthServer
}

func (h *HealthService) Check(ctx context.Context, req *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	return &healthpb.HealthCheckResponse{
		Status: healthpb.HealthCheckResponse_SERVING,
	}, nil
}
