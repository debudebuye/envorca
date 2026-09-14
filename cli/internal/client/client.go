package client

// Package client is the CLI's only connection to the daemon. It carries no
// infrastructure logic; it dials the local IPC endpoint and calls the API.

import (
	"context"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	envorcav1 "envorca.dev/envorca/api/gen/go/envorca/v1"
	"envorca.dev/envorca/api/ipc"
)

// Dial opens a gRPC connection to the daemon over the local IPC endpoint.
func Dial(ctx context.Context, endpoint string) (*grpc.ClientConn, error) {
	return grpc.NewClient("passthrough:///envorca",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return ipc.DialContext(ctx, endpoint)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
}

// Probe reports whether the daemon is reachable and responding.
func Probe(ctx context.Context, endpoint string) bool {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	conn, err := Dial(ctx, endpoint)
	if err != nil {
		return false
	}
	defer conn.Close()
	ctx2, cancel2 := context.WithTimeout(ctx, 2*time.Second)
	defer cancel2()
	_, err = envorcav1.NewDaemonClient(conn).Ping(ctx2, &envorcav1.PingRequest{})
	return err == nil
}

// GetStatus returns the current environment status.
func GetStatus(ctx context.Context, conn *grpc.ClientConn, timeout time.Duration) (*envorcav1.EnvironmentStatus, error) {
	c, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return envorcav1.NewDaemonClient(conn).GetStatus(c, &envorcav1.GetStatusRequest{})
}

// GetRepairPlan returns the currently applicable repair actions.
func GetRepairPlan(ctx context.Context, conn *grpc.ClientConn, timeout time.Duration) (*envorcav1.RepairPlan, error) {
	c, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return envorcav1.NewDaemonClient(conn).GetRepairPlan(c, &envorcav1.GetRepairPlanRequest{})
}

// ExecuteRepair runs one repair action from the plan.
func ExecuteRepair(ctx context.Context, conn *grpc.ClientConn, actionID string, confirmed bool, timeout time.Duration) (*envorcav1.RepairOutcome, error) {
	c, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return envorcav1.NewDaemonClient(conn).ExecuteRepair(c, &envorcav1.ExecuteRepairRequest{
		ActionId:  actionID,
		Confirmed: confirmed,
	})
}

// Shutdown requests a graceful daemon stop.
func Shutdown(ctx context.Context, conn *grpc.ClientConn, timeout time.Duration) error {
	c, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	_, err := envorcav1.NewDaemonClient(conn).Shutdown(c, &envorcav1.ShutdownRequest{})
	return err
}
