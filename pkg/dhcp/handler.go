package dhcp

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	iplist "github.com/go-sense/go-sense/pkg/ip"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/clientv3/clientv3util"
	dhcp "github.com/krolaw/dhcp4"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const (
	handleTimeout = 1 * time.Second

	etcdPrefixLease = "dhcp::leases::"
)

type Handler struct {
	log *zap.SugaredLogger
	// own IP address
	ip net.IP

	kv clientv3.KV

	freeIPs *iplist.List

	itnLock *sync.Mutex
	ipToNic map[string]string
	options dhcp.Options

	leaseDuration time.Duration
}

func getLeaseKey(nic string) string {
	return etcdPrefixLease + nic
}

func getNicFromLeaseKey(key string) string {
	return strings.TrimPrefix(key, etcdPrefixLease)
}

func New(kv clientv3.KV, cidr string, ownIP net.IP, leaseDuration time.Duration, log *zap.SugaredLogger, options dhcp.Options) (*Handler, error) {
	gresp, err := kv.Get(context.Background(), "dhcp::leases::", clientv3.WithPrefix())
	if err != nil {
		return nil, errors.Wrap(err, "failed to do the initial list of the existing dhcp leases")
	}

	freeIPs, err := iplist.ListFromCIDR(cidr)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to create IP list for CIDR '%s'", cidr))
	}
	log.Infof("Total IP's in CIDR: %d", freeIPs.Len())

	clients := map[string]string{}
	for _, kvs := range gresp.Kvs {
		nic := getNicFromLeaseKey(string(kvs.Key))
		ip := net.ParseIP(string(kvs.Value))
		clients[ip.String()] = nic

		freeIPs.Remove(ip)
	}
	freeIPs.Remove(ownIP)
	log.Infof("Clients with active leases: %d", len(clients))
	log.Infof("Free IP's in CIDR: %d", freeIPs.Len())

	return &Handler{
		itnLock:       &sync.Mutex{},
		ipToNic:       clients,
		kv:            kv,
		freeIPs:       freeIPs,
		leaseDuration: leaseDuration,
		ip:            ownIP,
		log:           log,
		options:       options,
	}, nil
}

func (h *Handler) ServeDHCP(p dhcp.Packet, msgType dhcp.MessageType, options dhcp.Options) dhcp.Packet {
	ctx, cancel := context.WithTimeout(context.Background(), handleTimeout)
	defer cancel()

	nic := p.CHAddr().String()
	log := h.log.With("ni", nic)

	switch msgType {
	case dhcp.Discover:
		log.Info("handling discover")

		freeIP, err := h.getNextFreeIP()
		if err != nil {
			log.Errorf("failed to get free IP: %v", err)
			return nil
		}

		return dhcp.ReplyPacket(p, dhcp.Offer, h.ip, freeIP, handleTimeout, h.options.SelectOrderOrAll(options[dhcp.OptionParameterRequestList]))

	case dhcp.Request:
		log.Info("handling request")

		if server, ok := options[dhcp.OptionServerIdentifier]; ok && !net.IP(server).Equal(h.ip) {
			return nil // Message not for this dhcp server
		}
		reqIP := net.IP(options[dhcp.OptionRequestedIPAddress])
		if reqIP == nil {
			reqIP = net.IP(p.CIAddr())
		}
		log = log.With("ip", reqIP.String())

		h.allocateIP(ctx, nic, reqIP)

		log.Info("leased IP %s to %v", reqIP.String(), nic)
		return dhcp.ReplyPacket(p, dhcp.ACK, h.ip, reqIP, h.leaseDuration, h.options.SelectOrderOrAll(options[dhcp.OptionParameterRequestList]))

	case dhcp.Release, dhcp.Decline:
		log.Info("handling release/decline")
		h.releaseIP(ctx, nic)
	}
	return nil
}

func (h *Handler) getNextFreeIP() (net.IP, error) {
	if h.freeIPs.Len() == 0 {
		return nil, errors.New("no free IP's left")
	}

	return h.freeIPs.First()
}

func (h *Handler) isFreeIP(ip net.IP) bool {
	return h.freeIPs.Contains(ip)
}

func (h *Handler) allocateIP(ctx context.Context, nic string, ip net.IP) error {
	if !h.freeIPs.Contains(ip) {
		return errors.New("ip is already allocated")
	}

	leaseKey := getLeaseKey(nic)

	txn := h.kv.Txn(ctx)
	txn.If(
		clientv3util.KeyMissing(leaseKey),
	).Then(
		clientv3.OpPut(leaseKey, ip.String()),
	)

	if _, err := txn.Commit(); err != nil {
		return errors.Wrap(err, "failed to store lease")
	}

	resp, err := h.kv.Get(ctx, leaseKey)
	if err != nil {
		return errors.Wrap(err, "failed to load lease")
	}

	storedIP := string(resp.Kvs[0].Value)
	if storedIP != ip.String() {
		return fmt.Errorf("stored ip '%s' for key '%s' does not match wanted IP %s", storedIP, leaseKey, ip.String())
	}

	h.freeIPs.Remove(ip)
	h.addToIPtoNicMapping(nic, ip)

	return nil
}

func (h *Handler) releaseIP(ctx context.Context, nic string) error {
	leaseKey := getLeaseKey(nic)

	resp, err := h.kv.Get(ctx, leaseKey)
	if err != nil {
		return errors.Wrap(err, "failed to load lease")
	}

	ip := net.ParseIP(string(resp.Kvs[0].Value))

	if _, err := h.kv.Delete(ctx, leaseKey); err != nil {
		return errors.Wrap(err, "failed to remove lease from store")
	}

	if err := h.freeIPs.Add(ip); err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to put IP %s back to list of free IP's", ip.String()))
	}
	h.removeFromPtoNicMapping(ip)

	return nil
}

func (h *Handler) addToIPtoNicMapping(nic string, ip net.IP) {
	h.itnLock.Lock()
	defer h.itnLock.Unlock()
	h.ipToNic[ip.String()] = nic
}

func (h *Handler) removeFromPtoNicMapping(ip net.IP) {
	h.itnLock.Lock()
	defer h.itnLock.Unlock()
	delete(h.ipToNic, ip.String())
}
