package ip

import (
	golist "container/list"
	"errors"
	"net"
	"sync"

	"github.com/krolaw/dhcp4"
)

type List struct {
	ipNet *net.IPNet
	// A linked list to have a O(1) complexity for modifying the list
	internalList *golist.List

	ipIndexLock *sync.Mutex
	// A index to have a O(1) complexity for element lookups
	ipIndex map[string]*golist.Element
}

func ListFromCIDR(cidr string) (*List, error) {
	ip, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, err
	}

	lookupMap := map[string]*golist.Element{}
	freeIPs := golist.New()
	for ip := ip.Mask(ipNet.Mask); ipNet.Contains(ip); inc(ip) {
		s := ip.String()
		lookupMap[s] = freeIPs.PushBack(net.ParseIP(s))
	}

	list := &List{
		internalList: freeIPs,
		ipIndex:      lookupMap,
		ipIndexLock:  &sync.Mutex{},
		ipNet:        ipNet,
	}

	// remove subnet address
	list.Remove(freeIPs.Front().Value.(net.IP))
	// remove broadcast address
	list.Remove(freeIPs.Back().Value.(net.IP))

	return list, nil
}

func (l *List) Remove(ip net.IP) {
	l.ipIndexLock.Lock()
	defer l.ipIndexLock.Unlock()

	s := ip.String()
	if e, exists := l.ipIndex[s]; exists {
		delete(l.ipIndex, s)
		l.internalList.Remove(e)
	}
}

func (l *List) Add(rip net.IP) error {
	// A IP is a slice, thus we should copy it to avoid any reference issues, as slice are passed by reference
	ip := copyIP(rip)

	if !l.ipNet.Contains(ip) {
		return errors.New("ip does not exist in list cidr")
	}

	insert := func(beforeElement *golist.Element) {
		l.ipIndexLock.Lock()
		defer l.ipIndexLock.Unlock()
		l.ipIndex[ip.String()] = l.internalList.InsertBefore(ip, beforeElement)
	}

	// This might be very expensive
	for e := l.internalList.Front(); e != nil; e = e.Next() {
		// Find the first IP which is bigger
		if !dhcp4.IPLess(ip, e.Value.(net.IP)) {
			insert(e)
			return nil
		}
	}

	insert(l.internalList.Back())
	return nil
}

func (l *List) Contains(ip net.IP) bool {
	l.ipIndexLock.Lock()
	defer l.ipIndexLock.Unlock()

	_, exists := l.ipIndex[ip.String()]
	return exists
}

func (l *List) Len() int {
	return l.internalList.Len()
}

func (l *List) First() (net.IP, error) {
	if l.internalList.Len() == 0 {
		return nil, errors.New("list is empty")
	}
	return l.internalList.Front().Value.(net.IP), nil
}

func copyIP(srcIP net.IP) net.IP {
	dst := make([]byte, len(srcIP))
	copy(dst, srcIP)
	return dst
}

func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}
