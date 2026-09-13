package client

// Package client is the CLI's only connection to the daemon. It carries no
// infrastructure logic; it dials the local IPC endpoint and calls the API.

import (
	"context"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	runorkav1 "runorka.dev/runorka/api/gen/go/runorka/v1"
	"runorka.dev/runorka/api/ipc"
)

// Dial opens a gRPC connection to the daemon over the local IPC endpoint.
func Dial(ctx context.Context, endpoint string) (*grpc.ClientConn, error) {
	return grpc.NewClient("passthrough:///runorka",
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
	_, err = runorkav1.NewDaemonClient(conn).Ping(ctx2, &runorkav1.PingRequest{})
	return err == nil
}

// GetStatus returns the current environment status.
func GetStatus(ctx context.Context, conn *grpc.ClientConn, timeout time.Duration) (*runorkav1.EnvironmentStatus, error) {
	c, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return runorkav1.NewDaemonClient(conn).GetStatus(c, &runorkav1.GetStatusRequest{})
}

// GetRepairPlan returns the currently applicable repair actions.
func GetRepairPlan(ctx context.Context, conn *grpc.ClientConn, timeout time.Duration) (*runorkav1.RepairPlan, error) {
	c, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return runorkav1.NewDaemonClient(conn).GetRepairPlan(c, &runorkav1.GetRepairPlanRequest{})
}

// ExecuteRepair runs one repair action from the plan.
func ExecuteRepair(ctx context.Context, conn *grpc.ClientConn, actionID string, confirmed bool, timeout time.Duration) (*runorkav1.RepairOutcome, error) {
	c, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return runorkav1.NewDaemonClient(conn).ExecuteRepair(c, &runorkav1.ExecuteRepairRequest{
		ActionId:  actionID,
		Confirmed: confirmed,
	})
}

// Shutdown requests a graceful daemon stop.
func Shutdown(ctx context.Context, conn *grpc.ClientConn, timeout time.Duration) error {
	c, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	_, err := runorkav1.NewDaemonClient(conn).Shutdown(c, &runorkav1.ShutdownRequest{})
	return err
}
