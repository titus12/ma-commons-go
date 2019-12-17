package net

import (
	"fmt"
	"github.com/golang/protobuf/descriptor"
	"sync"
)

type ProtocolNumber struct {
	names map[string]int32
	ids   map[int32]string
	mu    sync.RWMutex
}

var protocolNumber ProtocolNumber

func init() {
	protocolNumber.init()
}

func (p *ProtocolNumber) init() {

}

func (p *ProtocolNumber) load(msg descriptor.Message) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	names := make(map[string]int32)
	ids := make(map[int32]string)
	_, md := descriptor.ForMessage(msg)
	for i := 0; i < len(md.Field); i++ {
		v := md.GetField()[i]
		if _, ok := names[v.GetName()]; ok {
			return fmt.Errorf("parse protocol number, repeated protocol id %v..", v.GetName())
		}
		names[v.GetName()] = v.GetNumber()
		ids[v.GetNumber()] = v.GetName()
	}
	p.names = names
	p.ids = ids
	return nil
}

func (p *ProtocolNumber) getId(name string) int32 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.names[name]
}

func (p *ProtocolNumber) getName(id int32) string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.ids[id]
}

func LoadProtocol(msg descriptor.Message) error {
	if msg == nil {
		return nil
	}
	return protocolNumber.load(msg)
}

func GetProtocolId(name string) int32 {
	return protocolNumber.getId(name)
}

func GetProtocolName(id int32) string {
	return protocolNumber.getName(id)
}
