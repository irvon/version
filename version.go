// Copyright 2016 by caixw, All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

// Package version 是一个通用的版本号解析工具，可以一个版本号字符串解析到一个结构体中。
//
// version 通过 struct tag 的相关定义来解析版本号字符串。包含了以下标签。
// - index 该字段对应的的编号，也是默认的解析顺序(0 是入口)，只能为整数，唯一；
// - type 该字段的类型，可以值为 number(数字)、string(字符串)；
// - route 表示当前字段的结束字符，以及对应的需要跳转到的索引值值。
// 比如以下定义的结构体：
//  type struct Version {
//      Major int    `version:"0,number,.1,+2"`
//      Minor int    `version:"1,number,.2"`
//      Build string `version:"2,number"`
//  }
// 在解析时，首先会拿到索引为 0 的字段，也就是 Major，然后对字符串进行
// 依次比较，如果碰到符号 `.` ，则将前面的字符串转换成数值保存 Major，
// 然后跳到索引号为 1 的 Minor，再次对后续的字符串进行依次比较；若碰到
// 的是字符 `+` 则跳到索引值为 2 的 Build 字段，依次对后续的字符进行比
// 较；若碰到结尾了，而直接结束。
// 具体的定义，可参考下自带的 SemVersion。
package version

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"unicode"
)

// 表示结构体字段的类型，版本号要嘛是字符串，要嘛是数值
const (
	fieldTypeNumber = iota
	fieldTypeString
)

// 对每个字段的描述
type field struct {
	name   string        // 字段名称
	Type   int           // 该字段的类型，数值或是字符串
	Routes map[byte]int  // 该字段的路由，根据不同的字符，会跳到不同的元素中解析
	Value  reflect.Value // 该字段的 reflect.Value 类型，方便设置值。
}

// Parse 解析版本号字符串到 obj 中。
func Parse(obj interface{}, ver string) error {
	fields, err := getFields(obj)
	if err != nil {
		return err
	}

	start := 0
	field := fields[0]
	for i := 0; i < len(ver)+1; i++ {
		var nextIndex int
		var found bool

		if i < len(ver) { // 未结束字符串
			b := ver[i]
			nextIndex, found = field.Routes[b]
			if !found {
				continue
			}
		}

		switch field.Type {
		case fieldTypeNumber:
			n, err := strconv.ParseInt(ver[start:i], 10, 64)
			if err != nil {
				return err
			}
			field.Value.SetInt(n)
		case fieldTypeString:
			field.Value.SetString(ver[start:i])
		default:
			return errors.New("未知道的 fieldType" + strconv.Itoa(field.Type))
		}

		i++ // 过滤掉当前字符
		start = i
		field = fields[nextIndex] // 下一个 field
	} // end for

	return nil
}

// 将 obj 的所有可导出字段转换成 field 的描述形式，并以数组形式返回。
func getFields(obj interface{}) (map[int]*field, error) {
	v := reflect.ValueOf(obj)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return nil, errors.New("参数 obj 只能是结构体")
	}
	t := v.Type()

	fields := make(map[int]*field, v.NumField())
	for i := 0; i < v.NumField(); i++ {
		name := t.Field(i).Name

		tags := strings.Split(t.Field(i).Tag.Get("version"), ",")
		if len(tags) < 2 {
			return nil, fmt.Errorf("字段[%v]缺少必要的标签元素", name)
		}

		// 不可导出
		if unicode.IsLower(rune(name[0])) {
			return nil, fmt.Errorf("字段[%v]标记了 version 标记，但无法导出", name)
		}

		// tags[0]
		index, err := strconv.Atoi(tags[0])
		if err != nil {
			return nil, err
		}
		if _, found := fields[index]; found {
			return nil, fmt.Errorf("字段[%v]的索引值[%v]已经存在", name, index)
		}

		// tags[1]
		field := &field{Routes: make(map[byte]int, 2), name: name}
		switch tags[1] {
		case "number":
			field.Type = fieldTypeNumber
		case "string":
			field.Type = fieldTypeString
		default:
			return nil, fmt.Errorf("字段[%v]包含无效的标签：%v", name, tags[1])
		}

		// tags[2...]
		for _, v := range tags[2:] {
			n, err := strconv.Atoi(v[1:])
			if err != nil {
				return nil, err
			}
			field.Routes[v[0]] = n
		}

		field.Value = v.Field(i)

		fields[index] = field
	}

	if err := checkFields(fields); err != nil {
		return nil, err
	}

	return fields, nil
}

// 检测每个元素中的路由项都能找到对应的元素。
func checkFields(fields map[int]*field) error {
	for _, field := range fields {
		for b, index := range field.Routes {
			if _, found := fields[index]; !found {
				return fmt.Errorf("字段[%v]对应的路由项[%v]的值不存在", field.name, b)
			}
		}
	}

	return nil
}
