package ip

import (
	"fmt"
	"net"
	"testing"
)

func BenchmarkListFromCIDR(b *testing.B) {
	cidrs := []string{
		"10.0.0.0/8",
		"10.0.0.0/16",
		"10.0.0.0/24",
	}

	for _, cidr := range cidrs {
		b.Run(fmt.Sprintf("CIDR: %s", cidr), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				if _, err := ListFromCIDR(cidr); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkContains(b *testing.B) {
	tests := []struct {
		name   string
		cidr   string
		ipList *List
		ip     net.IP
	}{
		{
			name: "big /8 network",
			cidr: "10.0.0.0/8",
		},
		{
			name: "mid /16 network",
			cidr: "10.0.0.0/16",
		},
		{
			name: "small /24 network",
			cidr: "10.0.0.0/24",
		},
	}
	for i, t := range tests {
		list, err := ListFromCIDR(t.cidr)
		if err != nil {
			b.Fatal(err)
		}
		tests[i].ipList = list
		tests[i].ip = list.internalList.Back().Value.(net.IP)
	}

	for _, test := range tests {
		b.Run(test.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				if !test.ipList.Contains(test.ip) {
					b.Error("ip not found")
				}
			}
		})
	}
}

func TestList_Contains(t *testing.T) {
	list, err := ListFromCIDR("10.0.0.0/24")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		ip     net.IP
		exists bool
	}{
		{
			name:   "subnet address does not exist",
			ip:     net.ParseIP("10.0.0.0"),
			exists: false,
		},
		{
			name:   "broadcast address does not exist",
			ip:     net.ParseIP("10.0.0.255"),
			exists: false,
		},
		{
			name:   "IP from different network does not exist",
			ip:     net.ParseIP("192.0.0.1"),
			exists: false,
		},
		{
			name:   "first IP exists",
			ip:     net.ParseIP("10.0.0.1"),
			exists: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			exists := list.Contains(test.ip)
			if exists && !test.exists {
				t.Errorf("Expected that IP '%s' does not exist, but the list tells it exists", test.ip.String())
			}
			if !exists && test.exists {
				t.Errorf("Expected that IP '%s' does exist, but the list tells it does not exist", test.ip.String())
			}
		})
	}
}

func TestList_AddAndRemove(t *testing.T) {
	list, err := ListFromCIDR("10.0.0.0/24")
	if err != nil {
		t.Fatal(err)
	}

	ip1 := net.ParseIP("10.0.0.1")
	list.Remove(ip1)
	if list.Contains(ip1) {
		t.Errorf("IP '%s' still exists after it has been removed", ip1.String())
	}

	if err := list.Add(ip1); err != nil {
		t.Fatalf("Failed to add IP '%s' to list: %v", ip1.String(), err)
	}
	if !list.Contains(ip1) {
		t.Errorf("IP '%s' does not exist after adding it to the list", ip1.String())
	}
}
