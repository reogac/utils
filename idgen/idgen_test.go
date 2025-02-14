package idgen

import (
	"fmt"
	"math"
	"testing"
)

func Test_U32(t *testing.T) {
	fmt.Printf("Test Uint32 generator\n")
	gen := NewGenerator[uint32](0, math.MaxUint32)
	var id uint32
	last := 10
	for i := 0; i <= last; i++ {
		id = gen.Allocate()
		fmt.Printf("outcome = %d\n", id)
	}
	if id != uint32(last) {
		t.Errorf("should be %d", last)
	}
}
