package zgraphql

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"

	"github.com/aws/aws-lambda-go/events"
	"github.com/graphql-go/graphql"
	"github.com/pkg/errors"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zreflect"
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

func getGraphQlTypeFromKind(parentFieldName string, s any) graphql.Output {
	//fmt.Println("getGraphQlTypeFromKind:", item.TypeName, item.Kind, item.FieldName)
	rval := reflect.ValueOf(s)
	if rval.Kind() == reflect.Slice {
		nt := reflect.ValueOf(s).Type().Elem()
		n := reflect.New(nt)
		o := getGraphQlTypeFromKind(parentFieldName, n.Interface())
		return graphql.NewList(o)
	}
	kind := zreflect.KindFromReflectKindAndType(rval.Kind(), rval.Type())
	switch kind {
	case zreflect.KindStruct:
		o, err := newObjectFromStructure(parentFieldName, s)
		if err != nil {
			err = zlog.NewError(err, "getGraphQlTypeFromKind: struct", err, rval.Type())
			fmt.Println("zreflect.KindStruct err:", err)
			return nil
		}
		return o

	case zreflect.KindInt:
		return graphql.Int
	case zreflect.KindFloat:
		return graphql.Float
	case zreflect.KindString:
		return graphql.String
	case zreflect.KindBool:
		return graphql.Boolean
	case zreflect.KindTime:
		return graphql.DateTime
	default:
		//		fmt.Println("KIND:", item)
		return graphql.String
	}
}

func NewObjectFromStruct(s any) (object graphql.Type, err error) {
	// root, err := zreflect.IterateStruct(v, zreflect.Options{UnnestAnonymous: true})
	// if err != nil {
	// 	err = zlog.Error(err, "NewObjectFromStruct: urefect.IterateStruct", err)
	// }
	return newObjectFromStructure("", s)
}

func getInfoFromTag(fieldName, stags string) (name, description string, omitEmpty, ignore bool) {
	name = fieldName
	tags := zreflect.GetTagAsMap(stags)
	for key, vals := range tags {
		if key == "json" {
			for i, n := range vals {
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
		if key == "graphqldesc" && len(tags) == 1 {
			description = vals[0]
		}
		if key == "graphql" && len(tags) == 1 && vals[0] == "-" {
			ignore = true
		}
	}
	return
}

func getNonNullFromType(s any, t graphql.Output, omitEmpty bool) graphql.Output {
	if omitEmpty || reflect.ValueOf(s).Kind() == reflect.Pointer {
		return t
	}
	//	fmt.Println("non-null:", c.TypeName, c.FieldName, c.IsPointer, omitEmpty)
	return graphql.NewNonNull(t)
}

func newObjectFromStructure(parentFieldName string, s any) (object graphql.Type, err error) {
	//	fmt.Println("newObjectFromReflectItem:", item.TypeName, item.Kind, item.FieldName)
	var fields = graphql.Fields{}

	zreflect.ForEachField(s, zreflect.FlattenIfAnonymous, func(each zreflect.FieldInfo) bool {
		name, desc, omitEmpty, ignore := getInfoFromTag(each.StructField.Name, string(each.StructField.Tag))
		//		lowerName := ustr.MakeFirstLetterLowerCase(name)
		if ignore {
			return true
		}
		t := getGraphQlTypeFromKind(each.StructField.Name, s)
		t = getNonNullFromType(s, t, omitEmpty)
		field := &graphql.Field{
			Type:        t,
			Description: desc,
		}
		//		fmt.Println("addfield:", name, c.FieldName, c.Tag)
		fields[name] = field
		return true
	})
	name := parentFieldName
	if name == "" {
		name = reflect.TypeOf(s).Name()
		name = parentFieldName + "_" + name
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
