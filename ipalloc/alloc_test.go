package ipalloc

import (
	"fmt"
	"net"
	"testing"
)

func Test_Alloc1(t *testing.T) {
	_, cidr, _ := net.ParseCIDR("192.168.0.1/24")
	alloc := New(*cidr, 0, 1024)
	for i := 0; i < 100; i++ {
		alloc.Allocate()
	}
	if ip := alloc.Allocate(); ip == nil {
		t.Errorf("empty ip")
	} else if ip[3] == 101 {
		fmt.Printf("%s\n", ip)
		alloc.Release(ip)
	} else {
		t.Errorf("wrong ip address")
	}
}
func Test_Alloc2(t *testing.T) {
	_, cidr, _ := net.ParseCIDR("192.168.0.1/24")
	alloc := NewIpAllocator(*cidr, 50, 1)
	for i := 0; i < 30; i++ {
		alloc.Allocate()
	}
	if ip := alloc.Allocate(); ip == nil {
		t.Errorf("empty ip")
	} else if ip[3] == 81 {
		fmt.Printf("%s\n", ip)
		alloc.Release(ip)
	} else {
		t.Errorf("wrong ip address")
	}
}
