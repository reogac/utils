package ipalloc

import (
	"net"
	"sync"
)

type idPool struct {
	l     int64
	u     int64
	used  map[int64]bool
	mutex sync.Mutex
}

func newIdPool(l int64, u int64) idPool {
	return idPool{
		l:    l,
		u:    u,
		used: make(map[int64]bool),
	}
}

func (p *idPool) allocate() (id int64) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	/* This deterministic allocation scheme cause duplication in the case that
	* multiple SMFs are serving the same DN there is no mechanism for UPF to
	* allocate IP ranges for SMFs
	 */
	for id = p.l; id <= p.u; id++ {
		if _, ok := p.used[id]; !ok {
			p.used[id] = true
			return
		}
	}

	id = p.l - 1
	return
}

func (p *idPool) release(id int64) {
	p.mutex.Lock()
	delete(p.used, id)
	p.mutex.Unlock()
}

type IpAllocator struct {
	baseIp net.IP
	pool   idPool
}

func NewIpAllocator(cidr net.IPNet, ipRange int64, rangeIndex uint8) *IpAllocator {
	maxIp := MaxIp(cidr)
	l := int64(rangeIndex)*ipRange + 1
	if ipRange <= 0 || l > maxIp {
		return nil
	}
	u := l + ipRange - 1
	if u > maxIp {
		u = maxIp
	}

	return &IpAllocator{
		baseIp: cidr.IP,
		pool:   newIdPool(l, u),
	}
}

func New(cidr net.IPNet, l, u int64) *IpAllocator {
	maxIp := MaxIp(cidr)
	if u > maxIp {
		u = maxIp
	}
	if l >= u {
		l = u
	}
	if l <= 1 {
		l = 1
	}
	return &IpAllocator{
		baseIp: cidr.IP,
		pool:   newIdPool(l, u),
	}
}

// Allocate will allocate the IP address and returns it
func (a *IpAllocator) Allocate() (ip net.IP) {
	if id := a.pool.allocate(); id < a.pool.l {
		return
	} else {
		return offsetIp(a.baseIp, int(id))
	}
}

func (a *IpAllocator) Release(ip net.IP) {
	id := ipOffset(ip, a.baseIp)
	a.pool.release(int64(id))
}

// return new Ip by offseting a base Ip
func offsetIp(base net.IP, offset int) (ip net.IP) {
	ip = make(net.IP, len(base))
	copy(ip, base)

	var carry int
	for i := len(ip) - 1; i >= 0; i-- {
		if offset == 0 {
			break
		}

		val := int(ip[i]) + carry + offset%256
		ip[i] = byte(val % 256)
		carry = val / 256

		offset /= 256
	}

	return
}

// difference between 2 Ip addresses
func ipOffset(in, base net.IP) (offset int) {
	exp := 1
	for i := len(base) - 1; i >= 0; i-- {
		offset += int(in[i]-base[i]) * exp
		exp *= 256
	}
	return
}

func MaxIp(cidr net.IPNet) int64 {
	//calculate number of mask bits
	var bits int
	for _, b := range cidr.Mask {
		for ; b != 0; b /= 2 {
			if b%2 != 0 {
				bits++
			}
		}
	}
	return int64(1<<int64(32-bits)) - 2
}
