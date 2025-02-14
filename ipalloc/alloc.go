package ipalloc

import (
	"net"
	"sync"
)

const (
	NUM_TRIES int = 1024
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

	/*
		//NOTE: for now we have to use random allocator to deal with the duplication
		//caused by multiple SMFs. There must be another network function that
		//allocates IP range for SMF

		for i := 0; i < NUM_TRIES; i++ {
			id = rand.Int63n(p.u-p.l) + p.l
			if _, ok := p.used[id]; !ok {
				p.used[id] = true
				return
			}
		}
	*/
	id = p.l - 1
	return
}

func (p *idPool) release(id int64) {
	p.mutex.Lock()
	delete(p.used, id)
	p.mutex.Unlock()
}

type IpAllocator struct {
	cidr net.IPNet
	pool idPool
}

func New(cidr net.IPNet, l, u int64) *IpAllocator {
	//calculate number of mask bits
	var bits int
	for _, b := range cidr.Mask {
		for ; b != 0; b /= 2 {
			if b%2 != 0 {
				bits++
			}
		}
	}
	maxIp := int64(1<<int64(32-bits)) - 2
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
		cidr: cidr,
		pool: newIdPool(l, u),
	}
}

// Allocate will allocate the IP address and returns it
func (a *IpAllocator) Allocate() (ip net.IP) {
	if id := a.pool.allocate(); id < a.pool.l {
		return
	} else {
		return offsetIp(a.cidr.IP, int(id))
	}
}

func (a *IpAllocator) Release(ip net.IP) {
	id := ipOffset(ip, a.cidr.IP)
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
