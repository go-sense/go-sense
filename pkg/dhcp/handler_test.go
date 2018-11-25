package dhcp

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/coreos/etcd/integration"
	"github.com/go-sense/go-sense/pkg/log"
	dhcp "github.com/krolaw/dhcp4"
	"go.uber.org/zap"
)

const (
	testMAC = "00:28:f8:d0:90:35"
)

func TestAllocateIP(t *testing.T) {
	ec := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 1})
	defer ec.Terminate(t)

	client := ec.RandClient()

	logger, flush, err := log.GetLogger(zap.DebugLevel)
	if err != nil {
		t.Fatal(err)
	}
	defer flush()

	handler, err := New(client.KV, "10.0.0.0/24", net.ParseIP("10.0.0.1"), 12*time.Hour, logger, dhcp.Options{})
	if err != nil {
		t.Fatal(err)
	}

	wantIP := net.ParseIP("10.0.0.2")
	if err := handler.allocateIP(context.Background(), testMAC, wantIP); err != nil {
		t.Fatal(err)
	}

	if handler.freeIPs.Contains(wantIP) {
		t.Fatalf("wanted IP is still in list of free IP's after it has been allocated")
	}

	if wantNic := handler.ipToNic[wantIP.String()]; wantNic != testMAC {
		t.Fatalf("IP-NIC mapping does not contain the mapping for the before allocated IP '%s'", wantIP.String())
	}
}

func TestReleaseIP(t *testing.T) {
	ec := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 1})
	defer ec.Terminate(t)

	client := ec.RandClient()

	logger, flush, err := log.GetLogger(zap.DebugLevel)
	if err != nil {
		t.Fatal(err)
	}
	defer flush()

	handler, err := New(client.KV, "10.0.0.0/24", net.ParseIP("10.0.0.1"), 12*time.Hour, logger, dhcp.Options{})
	if err != nil {
		t.Fatal(err)
	}

	ip := net.ParseIP("10.0.0.2")
	if err := handler.allocateIP(context.Background(), testMAC, ip); err != nil {
		t.Fatal(err)
	}

	if err := handler.releaseIP(context.Background(), testMAC); err != nil {
		t.Fatal(err)
	}

	if _, found := handler.ipToNic[ip.String()]; found {
		t.Fatalf("IP-NIC mapping contains a mapping after the IP '%s' got released", ip.String())
	}

	if !handler.isFreeIP(ip) {
		t.Fatalf("IP '%s' is not in the list of free IP's after it has been released", ip.String())
	}
}
