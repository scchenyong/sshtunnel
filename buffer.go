package sshtunnel

import (
	"errors"
	"math"
	"sync"
)

type BufferPool struct {
	buffers []sync.Pool
}

func NewBufferPool() *BufferPool {
	alloc := new(BufferPool)
	alloc.buffers = make([]sync.Pool, 17)
	for k := range alloc.buffers {
		i := k
		alloc.buffers[k].New = func() interface{} {
			return make([]byte, 1<<uint16(i))
		}
	}
	return alloc
}

func (p *BufferPool) Get(size uint16) []byte {
	if size <= 0 || size > math.MaxUint16 {
		return nil
	}
	bits := msb(size)
	if size == 1<<bits {
		return p.buffers[bits].Get().([]byte)[:size]
	} else {
		return p.buffers[bits+1].Get().([]byte)[:size]
	}
}

func (p *BufferPool) Put(buf []byte) error {
	bc := cap(buf)
	if bc == 0 {
		return errors.New("放回的空间不能为空")
	}
	if bc > math.MaxUint16 {
		return errors.New("放回的空间超出缓存分配范围")
	}
	bits := msb(uint16(bc))
	p.buffers[bits].Put(buf)
	return nil
}

func msb(size uint16) uint16 {
	var pos uint16
	size >>= 1
	for size > 0 {
		size >>= 1
		pos++
	}
	return pos
}
