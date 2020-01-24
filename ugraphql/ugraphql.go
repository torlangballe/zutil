package ugraphql

import (
	"github.com/torlangballe/zutil/zreflect"
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-lambda-go/events"
	"github.com/graphql-go/graphql"
	"github.com/pkg/errors"
	"net/http"
)

func Handler(context context.Context, schema graphql.Schema, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	if len(request.Body) < 1 {
		return events.APIGatewayProxyResponse{}, errors.Errorf("No body")
	}

	result := graphql.Do(graphql.Params{
		Schema:        schema,
		RequestString: string(request.Body),
	})

	code := 200
	if result.HasErrors() {
		code = http.StatusInternalServerError
	}
	body, _ := json.Marshal(result)
	//	fmt.Println("body:", string(body))
	return events.APIGatewayProxyResponse{
		Body:       string(body),
		StatusCode: code,
	}, nil

}

func getGraphQlTypeFromKind(parentFieldName string, item ureflect.Item) graphql.Output {
	//fmt.Println("getGraphQlTypeFromKind:", item.TypeName, item.Kind, item.FieldName)
	if item.IsArray {
		var n = item
		n.IsArray = false
		o := getGraphQlTypeFromKind(parentFieldName, n)
		return graphql.NewList(o)
	}
	switch item.Kind {
	case ureflect.KindStruct:
		o, err := newObjectFromReflectItem(parentFieldName, item)
		if err != nil {
			err = errors.WrapN(err, "getGraphQlTypeFromKind: struct", err, item.TypeName)
			fmt.Println("ureflect.KindStruct err:", err)
			return nil
		}
		return o

	case ureflect.KindInt:
		return graphql.Int
	case ureflect.KindFloat:
		return graphql.Float
	case ureflect.KindString:
		return graphql.String
	case ureflect.KindBool:
		return graphql.Boolean
	case ureflect.KindTime:
		return graphql.DateTime
	default:
		//		fmt.Println("KIND:", item)
		return graphql.String
	}
}

func NewObjectFromStruct(v interface{}) (object graphql.Type, err error) {
	unnestAnonymous := true
	root, err := ureflect.ItterateStruct(v, unnestAnonymous)
	if err != nil {
		err = errors.WrapN(err, "NewObjectFromStruct: urefect.ItterateStruct", err)
	}
	return newObjectFromReflectItem("", root)
}

func getInfoFromTag(fieldName, tag string) (name, description string, omitEmpty, ignore bool) {
	name = fieldName
	for _, t := range ureflect.GetTagAsFields(tag) {
		if t.Label == "json" {
			for i, n := range t.Vars {
				if i == 0 {
					if n == "-" {
						ignore = true
						return
					}
					name = n
				} else if n == "omitempty" {
					omitEmpty = true
				}
			}
		}
		if t.Label == "graphqldesc" && len(t.Vars) == 1 {
			description = t.Vars[0]
		}
		if t.Label == "graphql" && len(t.Vars) == 1 && t.Vars[0] == "-" {
			ignore = true
		}
	}
	return
}

func getNonNullFromType(c ureflect.Item, t graphql.Output, omitEmpty bool) graphql.Output {
	if c.IsPointer || omitEmpty {
		return t
	}
	//	fmt.Println("non-null:", c.TypeName, c.FieldName, c.IsPointer, omitEmpty)
	return graphql.NewNonNull(t)
}

func newObjectFromReflectItem(parentFieldName string, item ureflect.Item) (object graphql.Type, err error) {
	//	fmt.Println("newObjectFromReflectItem:", item.TypeName, item.Kind, item.FieldName)
	var fields = graphql.Fields{}
	for _, c := range item.Children {

		name, desc, omitEmpty, ignore := getInfoFromTag(c.FieldName, c.Tag)
		//		lowerName := zstr.MakeFirstLetterLowerCase(name)
		if ignore {
			continue
		}
		t := getGraphQlTypeFromKind(item.TypeName, c)
		t = getNonNullFromType(c, t, omitEmpty)
		field := &graphql.Field{
			Type:        t,
			Description: desc,
		}
		//		fmt.Println("addfield:", name, c.FieldName, c.Tag)
		fields[name] = field
	}
	name := item.TypeName
	if name == "" {
		name = parentFieldName + "_" + item.FieldName
	}
	object = graphql.NewObject(graphql.ObjectConfig{
		Name:   name,
		Fields: fields,
	})
	return
}

func MakeEnum(name, description string, items ...EnumItem) graphql.Output {
	vals := graphql.EnumValueConfigMap{}
	for _, item := range items {
		c := item.Config
		vals[item.Name] = &c
	}
	return graphql.NewEnum(graphql.EnumConfig{
		Name:        name,
		Values:      vals,
		Description: description})
}

type EnumItem struct {
	Config graphql.EnumValueConfig
	Name   string
}

func Enum(val interface{}, name, description, deprecationReason string) EnumItem {
	return EnumItem{
		Name: name,
		Config: graphql.EnumValueConfig{
			Value:             val,
			Description:       description,
			DeprecationReason: deprecationReason,
		},
	}
}
