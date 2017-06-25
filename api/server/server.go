package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/quilt/quilt/api"
	"github.com/quilt/quilt/api/client"
	"github.com/quilt/quilt/api/pb"
	"github.com/quilt/quilt/counter"
	"github.com/quilt/quilt/db"
	"github.com/quilt/quilt/stitch"
	"github.com/quilt/quilt/version"

	"github.com/docker/distribution/reference"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	log "github.com/Sirupsen/logrus"
)

type server struct {
	conn db.Conn

	// The API server runs in two locations:  on minions in the cluster, and on
	// the daemon. When the server is running on the daemon, we automatically
	// proxy certain Queries to the cluster because the daemon doesn't track
	// those tables (e.g. Container, Connection, Label).
	runningOnDaemon bool
}

// Run starts a server that responds to `quiltctl` connections. It runs on both
// the daemon and on the minion. The server provides various client-relevant
// methods, such as starting deployments, and querying the state of the system.
// This is in contrast to the minion server (minion/pb/pb.proto), which facilitates
// the actual deployment.
func Run(conn db.Conn, listenAddr string, runningOnDaemon bool) error {
	proto, addr, err := api.ParseListenAddress(listenAddr)
	if err != nil {
		return err
	}

	var sock net.Listener
	apiServer := server{conn, runningOnDaemon}
	for {
		sock, err = net.Listen(proto, addr)

		if err == nil {
			break
		}
		log.WithError(err).Error("Failed to open socket.")

		time.Sleep(30 * time.Second)
	}

	// Cleanup the socket if we're interrupted.
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, os.Kill, syscall.SIGTERM, syscall.SIGHUP)
	go func(c chan os.Signal) {
		sig := <-c
		log.Printf("Caught signal %s: shutting down.\n", sig)
		sock.Close()
		os.Exit(0)
	}(sigc)

	s := grpc.NewServer()
	pb.RegisterAPIServer(s, apiServer)
	s.Serve(sock)

	return nil
}

// Query runs in two modes: daemon, or local. If in local mode, Query simply
// returns the requested table from its local database. If in daemon mode,
// Query proxies certain table requests (e.g. Container and Connection) to the
// cluster. This is necessary because some tables are only used on the minions,
// and aren't synced back to the daemon.
func (s server) Query(cts context.Context, query *pb.DBQuery) (*pb.QueryReply, error) {
	var rows interface{}
	var err error

	table := db.TableType(query.Table)
	if s.runningOnDaemon {
		rows, err = queryFromDaemon(table, s.conn)
	} else {
		rows, err = queryLocal(table, s.conn)
	}

	if err != nil {
		return nil, err
	}

	json, err := json.Marshal(rows)
	if err != nil {
		return nil, err
	}

	return &pb.QueryReply{TableContents: string(json)}, nil
}

func queryLocal(table db.TableType, conn db.Conn) (interface{}, error) {
	switch table {
	case db.MachineTable:
		return conn.SelectFromMachine(nil), nil
	case db.ContainerTable:
		return conn.SelectFromContainer(nil), nil
	case db.EtcdTable:
		return conn.SelectFromEtcd(nil), nil
	case db.ConnectionTable:
		return conn.SelectFromConnection(nil), nil
	case db.LabelTable:
		return conn.SelectFromLabel(nil), nil
	case db.ClusterTable:
		return conn.SelectFromCluster(nil), nil
	default:
		return nil, fmt.Errorf("unrecognized table: %s", table)
	}
}

func queryFromDaemon(table db.TableType, conn db.Conn) (
	interface{}, error) {

	switch table {
	case db.MachineTable, db.ClusterTable:
		return queryLocal(table, conn)
	}

	var leaderClient client.Client
	leaderClient, err := newLeaderClient(conn.SelectFromMachine(nil))
	if err != nil {
		return nil, err
	}
	defer leaderClient.Close()

	switch table {
	case db.ContainerTable:
		return getClusterContainers(conn, leaderClient)
	case db.ConnectionTable:
		return leaderClient.QueryConnections()
	case db.LabelTable:
		return leaderClient.QueryLabels()
	default:
		return nil, fmt.Errorf("unrecognized table: %s", table)
	}
}

func (s server) QueryCounters(ctx context.Context, in *pb.CountersRequest) (
	*pb.CountersReply, error) {
	return &pb.CountersReply{Counters: counter.Dump()}, nil
}

func (s server) Deploy(cts context.Context, deployReq *pb.DeployRequest) (
	*pb.DeployReply, error) {

	stitch, err := stitch.FromJSON(deployReq.Deployment)
	if err != nil {
		return &pb.DeployReply{}, err
	}

	for _, c := range stitch.Containers {
		if _, err := reference.ParseAnyReference(c.Image.Name); err != nil {
			return &pb.DeployReply{}, fmt.Errorf("could not parse "+
				"container image %s: %s", c.Image.Name, err.Error())
		}
	}

	err = s.conn.Txn(db.ClusterTable).Run(func(view db.Database) error {
		cluster, err := view.GetCluster()
		if err != nil {
			cluster = view.InsertCluster()
		}

		cluster.Blueprint = stitch.String()
		view.Commit(cluster)
		return nil
	})
	if err != nil {
		return &pb.DeployReply{}, err
	}

	// XXX: Remove this error when the Vagrant provider is done.
	for _, machine := range stitch.Machines {
		if machine.Provider == db.Vagrant {
			err = errors.New("The Vagrant provider is still in development." +
				" The blueprint will continue to run, but" +
				" there may be some errors.")
			return &pb.DeployReply{}, err
		}
	}

	return &pb.DeployReply{}, nil
}

func (s server) Version(_ context.Context, _ *pb.VersionRequest) (
	*pb.VersionReply, error) {
	return &pb.VersionReply{Version: version.Version}, nil
}

func getClusterContainers(conn db.Conn, leaderClient client.Client) (interface{}, error) {
	leaderContainers, err := leaderClient.QueryContainers()
	if err != nil {
		return nil, err
	}

	workerContainers, err := queryWorkers(conn.SelectFromMachine(nil))
	if err != nil {
		return nil, err
	}

	return updateLeaderContainerAttrs(leaderContainers, workerContainers), nil
}

type queryContainersResponse struct {
	containers []db.Container
	err        error
}

// queryWorkers gets a client for all worker machines and returns a list of
// `db.Container`s on these machines.
func queryWorkers(machines []db.Machine) ([]db.Container, error) {
	var wg sync.WaitGroup
	queryResponses := make(chan queryContainersResponse, len(machines))
	for _, m := range machines {
		if m.PublicIP == "" || m.Role != db.Worker {
			continue
		}

		wg.Add(1)
		go func(m db.Machine) {
			defer wg.Done()
			var qContainers []db.Container
			client, err := newClient(api.RemoteAddress(m.PublicIP))
			if err == nil {
				defer client.Close()
				qContainers, err = client.QueryContainers()
			}
			queryResponses <- queryContainersResponse{qContainers, err}
		}(m)
	}

	wg.Wait()
	close(queryResponses)

	var containers []db.Container
	for resp := range queryResponses {
		if resp.err != nil {
			return nil, resp.err
		}
		containers = append(containers, resp.containers...)
	}
	return containers, nil
}

// updateLeaderContainerAttrs updates the containers described by the leader with
// the worker-only attributes.
func updateLeaderContainerAttrs(lContainers []db.Container, wContainers []db.Container) (
	allContainers []db.Container) {

	// Map StitchID to db.Container for a hash join.
	cMap := make(map[string]db.Container)
	for _, wc := range wContainers {
		cMap[wc.StitchID] = wc
	}

	// If we are able to match a worker container to a leader container, then we
	// copy the worker-only attributes to the leader view.
	for _, lc := range lContainers {
		if wc, ok := cMap[lc.StitchID]; ok {
			lc.Created = wc.Created
			lc.DockerID = wc.DockerID
			lc.Status = wc.Status
		}
		allContainers = append(allContainers, lc)
	}
	return allContainers
}

// client.New and client.Leader are saved in variables to facilitate
// injecting test clients for unit testing.
var newClient = client.New
var newLeaderClient = client.Leader
