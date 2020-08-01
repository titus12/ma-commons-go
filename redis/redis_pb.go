package redis

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"
)

type ProtoByte struct {
	base       interface{}
	expiration time.Duration
}

func (p *ProtoByte) Bind(o interface{}) {
	p.base = o
}

func (p *ProtoByte) Load(key string) error {
	reply, err := RedisGet(key)
	if err != nil {
		return err
	}

	bytes, err := reply.Bytes()
	if err != nil {
		return err
	}

	err = proto.Unmarshal(bytes, p.base.(proto.Message))
	if err != nil {
		return err
	}

	return nil
}

func (p *ProtoByte) Save(key string) error {
	bytes, err := proto.Marshal(p.base.(proto.Message))
	if err != nil {
		return err
	}

	err = RedisSet(key, bytes, p.expiration)
	if err != nil {
		return err
	}

	return nil
}

type ProtoHash struct {
	base interface{}
}

func (p *ProtoHash) Bind(o interface{}) {
	p.base = o
}

func (p *ProtoHash) Load(key string) (err error, desc string) {
	ov := reflect.ValueOf(p.base)
	if ov.IsNil() {
		err = fmt.Errorf("base %v is nil", key)
		return
	}

	var hashmap map[string]string
	hashmap, err = RedisHash(key)
	if err != nil {
		return
	}

	for key, value := range hashmap {
		fv := ov.Elem().FieldByName(key)
		if !fv.IsValid() {
			desc += fmt.Sprintf("%v.%v invalid", key, key)
			// 兼容废弃字段
			continue
		}

		err = p.parse(fv, value)
		if err != nil {
			err = fmt.Errorf("parse %v.%v err %v", key, key, err)
			return
		}
	}

	return
}

func (p *ProtoHash) Save(key string) (err error, desc string) {
	rf := reflect.ValueOf(p.base)
	ov := reflect.Indirect(rf)

	for i := 0; i < ov.NumField(); i++ {
		name := ov.Type().Field(i).Name
		if name == "XXX_unrecognized" {
			continue
		}

		err = p.SaveField(name)
		if err != nil {
			desc += fmt.Sprintf("%v save err %v;", key, err)
			// 保存其他字段
			continue
		}
	}

	return
}

func (p *ProtoHash) LoadField(key string, name string) (err error) {
	ov := reflect.ValueOf(p.base)
	if ov.IsNil() {
		err = fmt.Errorf("base %v is nil", key)
		return
	}

	fv := ov.Elem().FieldByName(name)
	if !fv.IsValid() {
		err = fmt.Errorf("%v.%v invalid", key, name)
		return
	}

	var value string
	value, err = RedisHGet(key, name)
	if err != nil {
		if IsNilReply(err) {
			err = nil
			return
		}

		err = fmt.Errorf("RedisHGet %v.%v err %v", key, name, err)
		return
	}

	return p.parse(fv, value)
}

func (p *ProtoHash) LoadFields(key string, names ...string) (err error) {
	ov := reflect.ValueOf(p.base)
	if ov.IsNil() {
		err = fmt.Errorf("%v base is nil", key)
		return
	}

	var results []interface{}
	results, err = RedisHMGet(key, names...)
	if err != nil {
		err = fmt.Errorf("RedisHMGet %v.%v err %v", key, names, err)
		return
	}

	for i, name := range names {
		if results[i] == nil {
			continue
		}

		fv := ov.Elem().FieldByName(name)
		if !fv.IsValid() {
			err = fmt.Errorf("%v.%v invalid", key, name)
			return
		}

		value, ok := results[i].(string)
		if !ok {
			err = fmt.Errorf("%v.%v value not string", key, name)
			return
		}

		err = p.parse(fv, value)
		if err != nil {
			return
		}
	}

	return
}

func (p *ProtoHash) SaveField(key string, name string, fv interface{}) (err error) {
	var value string
	value, err = p.format(fv)
	if err != nil {
		return
	}

	_, err = RedisHSet(key, name, value)
	if err != nil {
		return
	}

	return nil
}

func (p *ProtoHash) parse(fieldValue reflect.Value, value string) error {
	ft := fieldValue.Type()
	if strings.Contains(ft.String(), "proto.") {
		if fieldValue.IsNil() {
			// New one
			v := reflect.New(ft.Elem())
			fieldValue.Set(v)
		}

		err := proto.Unmarshal([]byte(value), fieldValue.Interface().(proto.Message))
		if err != nil {
			return err
		}

		return nil
	}

	switch ft.String() {
	case "*int32":
		v, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return err
		}
		fieldValue.Set(reflect.ValueOf(proto.Int32(int32(v))))
	case "*int64":
		v, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return err
		}
		fieldValue.Set(reflect.ValueOf(proto.Int64(v)))
	case "*uint32":
		v, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return err
		}
		fieldValue.Set(reflect.ValueOf(proto.Uint32(uint32(v))))
	case "*uint64":
		v, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return err
		}
		fieldValue.Set(reflect.ValueOf(proto.Uint64(v)))
	case "*float32":
		v, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return err
		}
		fieldValue.Set(reflect.ValueOf(proto.Float32(float32(v))))
	case "*float64":
		v, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return err
		}
		fieldValue.Set(reflect.ValueOf(proto.Float64(float64(v))))
	case "*bytes":
		fallthrough
	case "*string":
		fieldValue.Set(reflect.ValueOf(proto.String(value)))
	case "*bool":
		v, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		fieldValue.Set(reflect.ValueOf(proto.Bool(v)))
	case "enum":
		//TODO
		/*
			v, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return err
			}
			en := int32(v)
			//fieldValue.Set(reflect.ValueOf(proto.Int32(int32(v))))
		*/
	default:
		return fmt.Errorf("type %v not handled", ft.String())
	}

	return nil
}

func (p *ProtoHash) format(fieldValue reflect.Value) (value string, err error) {
	ft := fieldValue.Type()
	if strings.Contains(ft.String(), "proto.") {
		if !fieldValue.IsNil() {
			var bytes []byte
			bytes, err = proto.Marshal(fieldValue.Interface().(proto.Message))
			if err != nil {
				return
			}

			value = string(bytes)
		}

		return
	}

	switch ft.String() {
	case "*int32":
		v := fieldValue.Interface().(*int32)
		value = strconv.FormatInt(int64(*v), 10)
		return
	case "*int64":
		v := fieldValue.Interface().(*int64)
		value = strconv.FormatInt(*v, 10)
		return
	case "*uint32":
		v := fieldValue.Interface().(*uint32)
		value = strconv.FormatUint(uint64(*v), 10)
		return
	case "*uint64":
		v := fieldValue.Interface().(*uint64)
		value = strconv.FormatUint(*v, 10)
		return
	case "*float32":
		v := fieldValue.Interface().(*float32)
		value = strconv.FormatFloat(float64(*v), 'E', -1, 32)
		return
	case "*float64":
		v := fieldValue.Interface().(*float64)
		value = strconv.FormatFloat(*v, 'E', -1, 64)
		return
	case "*bytes":
		v := fieldValue.Interface().(*string)
		value = *v
		return
	case "*string":
		v := fieldValue.Interface().(*string)
		value = *v
		return
	case "*bool":
		v := fieldValue.Interface().(*bool)
		value = strconv.FormatBool(*v)
		return
	default:
		err = fmt.Errorf("type %v not handled", ft.String())
	}

	return
}
