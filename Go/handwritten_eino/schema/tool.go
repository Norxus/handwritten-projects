package schema

import (
	"sort"

	"github.com/eino-contrib/jsonschema"
	orderedmap "github.com/wk8/go-ordered-map/v2"
)

type DataType string

const (
	Object  DataType = "object"
	Number  DataType = "number"
	Integer DataType = "integer"
	String  DataType = "string"
	Array   DataType = "array"
	Null    DataType = "null"
	Boolean DataType = "boolean"
)

type ToolChoice string

const (
	ToolChoiceForbidden ToolChoice = "forbidden"
	ToolChoiceAllowed   ToolChoice = "allowed"
	ToolChoiceForced    ToolChoice = "forced"
)

type ToolInfo struct {
	Name  string
	Desc  string
	Extra map[string]any
	// ParamsOneOf may be nil
	*ParamsOneOf
}

type ParameterInfo struct {
	Type      DataType
	// 数组元素类型
	ElemInfo  *ParameterInfo
	// 子字段类型
	SubParams map[string]*ParameterInfo
	Desc      string
	// 枚举限制类型
	Enum      []string
	Required  bool
}

type ParamsOneOf struct {
	params     map[string]*ParameterInfo
	jsonschema *jsonschema.Schema
}

// 提供两种方式创建
func NewParamsOneOfByParams(params map[string]*ParameterInfo) *ParamsOneOf {
	return &ParamsOneOf{
		params: params,
	}
}

func NewParamsOneOfByJSONSchema(s *jsonschema.Schema) *ParamsOneOf {
	return &ParamsOneOf{
		jsonschema: s,
	}
}

func (p *ParamsOneOf) ToJSONSchema() (*jsonschema.Schema, error) {
	if p == nil {
		return nil, nil
	}

	// 当 ParamsOneOf 里存的是 params map[string]*ParameterInfo 这种“简化参数描述”时
	// 把它 转换成标准的 JSON Schema 返回
	if p.params != nil {
		sc := &jsonschema.Schema{
			Properties: orderedmap.New[string, *jsonschema.Schema](),
			Type:       string(Object),
			Required:   make([]string, 0, len(p.params)),
		}

		keys := make([]string, 0, len(p.params))
		for k := range p.params {
			keys = append(keys, k)
		}
		// 因为 Go 的 map 遍历顺序是不稳定的，所以这里先排序，保证生成结果稳定、可预测，测试也更容易写
		sort.Strings(keys)

		for _, k := range keys {
			// 拿到这个参数的描述，也就是 *ParameterInfo
			v := p.params[k]
			sc.Properties.Set(k, paramInfoToJSONSchema(v))
			if v.Required {
				sc.Required = append(sc.Required, k)
			}
		}

		return sc, nil
	}

	return p.jsonschema, nil
}

// paramInfo -> json schema
func paramInfoToJSONSchema(paramInfo *ParameterInfo) *jsonschema.Schema {
	js := &jsonschema.Schema{
		Type:        string(paramInfo.Type),
		Description: paramInfo.Desc,
	}

	// json schema 的 enum 字段用于表示这个字段的值只能是这几个之一
	if len(paramInfo.Enum) > 0 {
		js.Enum = make([]any, len(paramInfo.Enum))
		for i, enum := range paramInfo.Enum {
			js.Enum[i] = enum
		}
	}

	// 在处理“数组元素的类型信息”，深入解析数组类型
	if paramInfo.ElemInfo != nil {
		js.Items = paramInfoToJSONSchema(paramInfo.ElemInfo)
	}

	// 当前参数下面还有子参数，说明当前参数是一个 object
	if len(paramInfo.SubParams) > 0 {
		required := make([]string, 0, len(paramInfo.SubParams))
		// 创建了一个“有序 map”，因为 Go 的普通 map 遍历顺序不稳定。
		// 这里用 orderedmap ，再配合前面的排序，就可以让生成出来的 properties 顺序稳定
		js.Properties = orderedmap.New[string, *jsonschema.Schema]()
		keys := make([]string, 0, len(paramInfo.SubParams))
		for k := range paramInfo.SubParams {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			v := paramInfo.SubParams[k]
			item := paramInfoToJSONSchema(v)
			js.Properties.Set(k, item)
			if v.Required {
				required = append(required, k)
			}
		}

		js.Required = required
	}

	return js
}
