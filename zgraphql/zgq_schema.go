package zgraphql

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/torlangballe/zutil/zhttp"
	"github.com/torlangballe/zutil/zlog"
)

var escBlack string = "\x1B[30m"
var escRed string = "\x1B[31m"
var escGreen string = "\x1B[32m"
var escYellow string = "\x1B[33m"
var escBlue string = "\x1B[34m"
var escMagenta string = "\x1B[35m"
var escCyan string = "\x1B[36m"
var escWhite string = "\x1B[37m"
var escNoColor = "\x1b[0m"

const introQuery = `
{
    __schema {
      queryType { name }
      mutationType { name }
      subscriptionType { name }
      types {
        ...FullType
      }
      directives {
        name
        description
		locations
        args {
          ...InputValue
        }
        # deprecated, but included for coverage till removed
		onOperation
        onFragment
        onField
      }
    }
  }

  fragment FullType on __Type {
    kind
    name
    description
    fields(includeDeprecated: true) {
      name
      description
      args {
        ...InputValue
      }
      type {
        ...TypeRef
      }
      isDeprecated
      deprecationReason
    }
    inputFields {
      ...InputValue
    }
    interfaces {
      ...TypeRef
    }
    enumValues(includeDeprecated: true) {
      name
      description
      isDeprecated
      deprecationReason
    }
    possibleTypes {
      ...TypeRef
    }
  }

  fragment InputValue on __InputValue {
    name
    description
    type { ...TypeRef }
    defaultValue
  }

  fragment TypeRef on __Type {
    kind
    name
    ofType {
      kind
      name
      ofType {
        kind
        name
        ofType {
          kind
          name
          ofType {
            kind
            name
            ofType {
              kind
              name
              ofType {
                kind
                name
                ofType {
                  kind
                  name
                }
              }
            }
          }
        }
      }
    }
  }
`

type InFieldType struct {
	Kind   string       `json:"kind"`
	Name   string       `json:"name"`
	OfType *InFieldType `json:"ofType"`
}

type OutType struct {
	Type        string
	NonNull     bool
	Depriciated bool
	IsList      bool
	IsArg       bool
	IsScalar    bool
	Description string
	Name        string
	IsEnum      bool
	Default     interface{}
	Children    []OutType
}

type EnumStruct struct {
	DeprecationReason string `json:"deprecationReason"`
	Description       string `json:"description"`
	IsDeprecated      bool   `json:"isDeprecated"`
	Name              string `json:"name"`
}

type InField struct {
	EnumStruct
	Type InFieldType `json:"type"`
	Kind string      `json:"kind"`
	Args []InArg     `json:"args"`
}

type InType struct {
	Description string       `json:"description"`
	Name        string       `json:"name"`
	Fields      []InField    `json:"fields"`
	EnumValues  []EnumStruct `json:"enumValues"`
	Kind        string       `json:"kind"`
}

type InArg struct {
	DefaultValue interface{} `json:"defaultValue"`
	Description  string      `json:"description"`
	Name         string      `json:"name"`
	Type         InFieldType `json:"type"`
}

type Result struct {
	Schema struct {
		QueryType struct {
		} `json:"queryType"`
		Types []InType `json:"types"`
	} `json:"__schema"`
}

func getFieldType(t InFieldType) OutType {
	var o OutType

	o.Type = t.Name
	if t.OfType != nil {
		o = getFieldType(*t.OfType)
	}
	switch t.Kind {
	case "NON_NULL":
		o.NonNull = true
	case "LIST":
		o.IsList = true
	}
	return o
}

func getField(f InField) OutType {
	var o OutType

	o.Name = f.Name
	o.Description = f.Description
	o.Depriciated = f.IsDeprecated
	if f.IsDeprecated {
		if o.Description != "" {
			o.Description += " • "
		}
		o.Description += f.DeprecationReason
	}
	ft := getFieldType(f.Type)
	o.Type = ft.Type
	o.IsList = ft.IsList
	o.NonNull = ft.NonNull

	for _, a := range f.Args {
		var n OutType
		n.Name = a.Name
		n.Description = a.Description
		n.Default = a.DefaultValue
		n.IsArg = true
		nft := getFieldType(a.Type)
		n.Type = nft.Type
		n.IsList = nft.IsList
		n.NonNull = nft.NonNull
		o.Children = append(o.Children, n)
	}
	return o
}

func getType(t InType) OutType {
	var o OutType

	o.Name = t.Name
	o.Description = t.Description

	for _, f := range t.Fields {
		o.Children = append(o.Children, getField(f))
	}
	switch t.Kind {
	case "ENUM":
		o.IsEnum = true
		for _, e := range t.EnumValues {
			var n OutType
			n.Type = "enum"
			n.Name = e.Name
			n.Depriciated = e.IsDeprecated
			n.Description = e.Description
			o.Children = append(o.Children, n)
		}
	case "SCALAR":
		o.IsScalar = true
	}

	return o
}

func getFieldStr(f OutType) string {
	stype := f.Type
	if f.IsList {
		stype = "[" + stype + "]"
	}
	if f.NonNull {
		stype += "!"
	}
	args := ""
	for _, a := range f.Children {
		if a.IsArg {
			if args != "" {
				args += ", "
			}
			args += getFieldStr(a)
		}
	}
	if args != "" {
		args = "(" + args + ")"
	}
	col := escCyan
	if f.Depriciated {
		col = escRed
	}
	return col + f.Name + escNoColor + args + ": " + escYellow + stype + escNoColor
}

func getComment(t OutType) (comment string) {
	if t.Depriciated {
		return escRed + " # depreciated. " + t.Description
	}
	if t.Description != "" {
		comment = escBlue + " # " + t.Description
	}
	args := ""
	for _, a := range t.Children {
		if a.IsArg && a.Description != "" {
			if args != "" {
				args += ", "
			}
			args += a.Name + ": " + a.Description
		}
	}
	if args != "" {
		if comment == "" {
			comment = escBlue + " # " + args
		} else {
			comment += " • " + args
		}
	}
	return
}

func printType(prefix string, t OutType) {
	stype := "type"
	if t.IsEnum {
		stype = "enum"
	}
	if t.IsScalar {
		stype = "scalar"
	}
	comment := getComment(t)
	if prefix != "" {
		fmt.Println(prefix+escMagenta+stype+escNoColor, "{", comment)
	} else {
		fmt.Println(prefix+escMagenta+stype+escNoColor, t.Name+" {", comment)
	}
	for _, f := range t.Children {
		if f.Type == "enum" {
			col := escGreen
			if f.Depriciated {
				col = escRed
			}
			fmt.Println(prefix+col+"  "+f.Name, getComment(f))
			continue
		}
		str := getFieldStr(f)
		str += getComment(f)
		fmt.Println(prefix + "  " + str)
	}
	fmt.Println(escNoColor + prefix + "}")
}

func postBytesSetContentLength(surl string, ctype string, body []byte) (response *http.Response, err error) {
	client := http.DefaultClient
	req, err := http.NewRequest("POST", surl, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Length", fmt.Sprintf("%d", len(body)))
	req.Header.Set("Content-Type", ctype)
	//	req.Header.Set("Content-Type", ctype)
	response, err = client.Do(req)
	if err != nil {
		return
	}
	return
}

func PostQueryToURL(resultPointer any, surl string, query string) error {
	params := zhttp.MakeParameters()
	params.PrintBody = true
	params.GetErrorFromBody = true
	params.ContentType = "application/json"
	var out struct {
		Query string `json:"query"`
	}
	var errorResult struct {
		Errors []struct {
			Message    string `json:"message"`
			Extensions struct {
				Category string `json:"category"`
			} `json:"extensions"`
			Locations []struct {
				Line   int `json:"line"`
				Column int `json:"column"`
			} `json:"locations"`
		} `json:"errors"`
	}
	out.Query = query
	var data []byte
	_, err := zhttp.Post(surl, params, out, &data)
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, resultPointer)
	zlog.Info("POST", err)
	if err == nil {
		err2 := json.Unmarshal(data, &errorResult)
		if err2 == nil && len(errorResult.Errors) > 0 {
			var errs []error
			for _, e := range errorResult.Errors {
				str := e.Message
				for _, loc := range e.Locations {
					str += " line: " + fmt.Sprint(loc.Line)
				}
				merr := zlog.NewError(str)
				errs = append(errs, merr)
			}
			return errors.Join(errs...)
		}
	}
	return err
}

func PrintSchemaFromURL(surl string, isInDataStruct, nest, debug bool) error {
	var data []byte
	err := PostQueryToURL(&data, surl, introQuery)
	if err != nil {
		return err
	}
	if debug {
		fmt.Println(string(data))
	}
	return PrintSchemaFromJson(data, isInDataStruct, nest)
}

func PrintSchemaFromJson(jsonData []byte, isInDataStruct, nest bool) error {
	var result Result
	var dataResult struct {
		Data Result `json:"data"`
	}
	var types []OutType

	var err error
	if isInDataStruct {
		err = json.Unmarshal(jsonData, &dataResult)
		result = dataResult.Data
	} else {
		err = json.Unmarshal(jsonData, &result)
	}
	if err != nil {
		fmt.Println("Unmarshal Error:", err)
		return err
	}
	for _, t := range result.Schema.Types {
		if !strings.HasPrefix(t.Name, "__") && t.Name != "String" && t.Name != "Boolean" && t.Name != "Float" && t.Name != "Int" {
			types = append(types, getType(t))
		}
	}
	for _, t := range types {
		printType("", t)
		fmt.Println("")
	}
	fmt.Println("")
	return nil
}
