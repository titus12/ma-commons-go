package net

import (
	"fmt"
	"github.com/golang/protobuf/descriptor"
	"sync"
)

type ProtocolCode struct {
	names map[string]int16
	ids   map[int16]string
	mu    sync.RWMutex
}

var defaultProtocolCode ProtocolCode

func NewProtocolCode(msg ...descriptor.Message) (*ProtocolCode, error) {
	p := &ProtocolCode{}
	err := p.load(msg...)
	return p, err
}

func (p *ProtocolCode) load(msg ...descriptor.Message) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.names = make(map[string]int16)
	p.ids = make(map[int16]string)
	for _, m := range msg {
		_, md := descriptor.ForMessage(m)
		for i := 0; i < len(md.Field); i++ {
			v := md.GetField()[i]
			if _, ok := p.names[v.GetName()]; ok {
				return fmt.Errorf("parse protocol code, repeated protocol id %v", v.GetName())
			}
			name := v.GetTypeName()[1:]
			p.names[name] = int16(v.GetNumber())
			p.ids[int16(v.GetNumber())] = name
		}
	}
	return nil
}

func (p *ProtocolCode) GetId(name string) int16 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.names[name]
}

func (p *ProtocolCode) GetName(id int16) string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.ids[id]
}

func (p *ProtocolCode) ReloadProtocol(msg ...descriptor.Message) error {
	if len(msg) <= 0 {
		return nil
	}
	return p.load(msg...)
}

func LoadProtocol(msg ...descriptor.Message) error {
	if msg == nil {
		return nil
	}
	return defaultProtocolCode.load(msg...)
}

func GetProtocolId(name string) int16 {
	return defaultProtocolCode.GetId(name)
}

func GetProtocolName(id int16) string {
	return defaultProtocolCode.GetName(id)
}
