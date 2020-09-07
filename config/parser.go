package config

import (
	"errors"
	"fmt"
	"reflect"
)

type structFieldType uint

const (
	typeInvalid structFieldType = iota
	typeField
	typeStruct
	typeSlice
)

type parser struct {
	dups map[Source]string
}

func newParser() *parser {
	return &parser{}
}

func (p *parser) ParseCfg(cfg interface{}) ([]*Field, error) {
	p.dups = make(map[Source]string)

	tp := reflect.TypeOf(cfg)
	if tp.Kind() != reflect.Ptr {
		return nil, errors.New("configuration should be a pointer type")
	}

	return p.getFields("", tp.Elem(), reflect.ValueOf(cfg).Elem())
}

func (p *parser) getFields(prefix string, tp reflect.Type, val reflect.Value) ([]*Field, error) {
	var ff []*Field
	for i := 0; i < tp.NumField(); i++ {
		f := tp.Field(i)

		typ, err := p.getStructFieldType(f, val.Field(i))
		if err != nil {
			return nil, err
		}

		switch typ {
		case typeField:
			fld, err := p.createField(prefix, f, val.Field(i))
			if err != nil {
				return nil, err
			}
			ff = append(ff, fld)
		case typeStruct:
			nested, err := p.getFields(prefix+f.Name, f.Type, val.Field(i))
			if err != nil {
				return nil, err
			}
			ff = append(ff, nested...)
		case typeSlice:
			fld, err := p.createSliceField(prefix, f, val.Field(i))
			if err != nil {
				return nil, err
			}
			ff = append(ff, fld)
		}
	}
	return ff, nil
}

func (p *parser) createField(prefix string, f reflect.StructField, val reflect.Value) (*Field, error) {
	fld := newField(prefix, f, val)

	value, ok := fld.Sources()[SourceConsul]
	if ok {
		if p.isKeyValueDuplicate(SourceConsul, value) {
			return nil, fmt.Errorf("duplicate value %v for source %s", fld, SourceConsul)
		}
	}

	return fld, nil
}

func (p *parser) createSliceField(prefix string, f reflect.StructField, val reflect.Value) (*Field, error) {
	fld := newSliceField(prefix, f, val)

	value, ok := fld.Sources()[SourceConsul]
	if ok {
		if p.isKeyValueDuplicate(SourceConsul, value) {
			return nil, fmt.Errorf("duplicate value %v for source %s", fld, SourceConsul)
		}
	}

	return fld, nil
}

func (p *parser) isKeyValueDuplicate(src Source, value string) bool {
	v, ok := p.dups[src]
	if ok {
		if value == v {
			return true
		}
	}
	p.dups[src] = value
	return false
}

func (p *parser) getStructFieldType(f reflect.StructField, val reflect.Value) (structFieldType, error) {
	t := f.Type
	fmt.Println(t.Kind().String())
	if t.Kind() != reflect.Struct && t.Kind() != reflect.Slice {
		return typeInvalid, fmt.Errorf("only struct type supported for %s", f.Name)
	}
	if t.Kind() == reflect.Slice {
		return typeSlice, nil
	}
	cfgType := reflect.TypeOf((*CfgType)(nil)).Elem()

	for _, tag := range sourceTags {
		if _, ok := f.Tag.Lookup(string(tag)); ok {
			if !val.Addr().Type().Implements(cfgType) {
				return typeInvalid, fmt.Errorf("field %s must implement CfgType interface", f.Name)
			}
			return typeField, nil
		}
	}

	return typeStruct, nil
}
