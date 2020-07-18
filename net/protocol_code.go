package net

import (
	"fmt"
	"github.com/golang/protobuf/descriptor"
	"sync"
)

type ProtocolNumber struct {
	names map[string]int16
	ids   map[int16]string
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
	names := make(map[string]int16)
	ids := make(map[int16]string)
	_, md := descriptor.ForMessage(msg)
	for i := 0; i < len(md.Field); i++ {
		v := md.GetField()[i]
		if _, ok := names[v.GetName()]; ok {
			return fmt.Errorf("parse protocol number, repeated protocol id %v..", v.GetName())
		}
		names[v.GetName()] = int16(v.GetNumber())
		ids[int16(v.GetNumber())] = v.GetName()
	}
	p.names = names
	p.ids = ids
	return nil
}

func (p *ProtocolNumber) getId(name string) int16 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.names[name]
}

func (p *ProtocolNumber) getName(id int16) string {
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

func GetProtocolId(name string) int16 {
	return protocolNumber.getId(name)
}

func GetProtocolName(id int16) string {
	return protocolNumber.getName(id)
}
