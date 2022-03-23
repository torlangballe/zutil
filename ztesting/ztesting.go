package ztesting

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/torlangballe/zutil/zdict"
	"github.com/torlangballe/zutil/zhtml"
	"github.com/torlangballe/zutil/zhttp"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zstr"
)

type UnmarshalFunc func(body []byte, to interface{}) error

func GetFromURL(to interface{}, surl string, unmarshal UnmarshalFunc) error {
	params := zhttp.MakeParameters()
	params.TimeoutSecs = 10
	// params.PrintBody = true
	var body []byte
	_, err := zhttp.Get(surl, params, &body)
	if err != nil {
		return zlog.NewError(err, surl)
	}
	err = unmarshal(body, to)
	if err != nil {
		text, herr := zhtml.ExtractTextFromHTMLString(string(body))
		if herr == nil && text != "" {
			text = zstr.Tail(text, 200)
			return errors.New(text)
		}
		return zlog.Error(err, "decode")
	}
	return nil
}

func GetItems(structure interface{}, path string, sliceMax int) (items zdict.Items) {
	val := reflect.ValueOf(structure)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	kind := val.Kind()
	vtype := val.Type()
	if kind == reflect.Struct {
		for i := 0; i < vtype.NumField(); i++ {
			fval := val.Field(i)
			if fval.Kind() == reflect.Ptr {
				fval = fval.Elem()
			}
			fkind := fval.Kind()
			ftype := vtype.Field(i)
			fname := ftype.Name
			if !strings.HasPrefix(fname, "XML") {
				npath := path
				if npath != "" {
					npath += "."
				}
				npath += fname
				if fkind == reflect.Struct {
					items = append(items, GetItems(fval.Interface(), npath, sliceMax)...)
				} else if fkind == reflect.Slice {
					for j := 0; j < fval.Len() && j <= sliceMax; j++ {
						items = append(items, GetItems(fval.Index(j).Interface(), npath+fmt.Sprintf("[%d]", j), sliceMax)...)
					}
				} else {
					items = append(items, zdict.Item{Name: npath, Value: fval})
				}
			}
		}
	} else if kind == reflect.Slice {
		items = append(items, GetItems(val.Index(0).Interface(), path, sliceMax)...)
	}
	return
}

func GetItemValueFromURL(structure interface{}, surl string, unmarshal UnmarshalFunc, sliceMax int, name string) (string, error) {
	items, err := GetItemsFromURL(structure, surl, unmarshal, sliceMax)
	if err != nil {
		return "", err
	}
	for _, item := range items {
		if item.Name == name {
			return fmt.Sprint(item.Value), nil
		}
	}
	return "", errors.New("not found")
}

func GetItemsFromURL(structure interface{}, surl string, unmarshal UnmarshalFunc, sliceMax int) (zdict.Items, error) {
	err := GetFromURL(structure, surl, unmarshal)
	if err != nil {
		return zdict.Items{}, err
	}
	return GetItems(structure, "", sliceMax), nil
	// for n, v := range items {
	// 	fmt.Printf("\"%s\": \"%v\",\n", n, v)
	// }
}

func matchOrEqual(wild, str string) bool {
	if !strings.Contains(wild, "*") {
		return wild == str
	}
	// wild = strings.ReplaceAll(wild, "[", `\[`)
	// wild = strings.ReplaceAll(wild, "]", `\]`)
	m := zstr.MatchWildcard(wild, str)
	// fmt.Println("matchOrEqual", wild, str, m)
	return m
}

func compareValues(name, tryVal, v, surl string) error {
	match := matchOrEqual(tryVal, v)
	// if match {
	// 	fmt.Println("comp:", name, tryVal, v, surl, match, err)
	// }
	if !match {
		showVal := v
		if strings.Contains(name, "*") {
			showVal = ""
		}
		return zlog.NewError(name, showVal, "!=", tryVal)
	}
	return nil
}

func FindValues(items zdict.Items, surl string, nameValues map[string]string) (errs []error) {
outer:
	for name, tryVal := range nameValues {
		var wildError error
		for _, item := range items {
			match := matchOrEqual(name, item.Name)
			if match {
				err := compareValues(name, tryVal, fmt.Sprint(item.Value), surl)
				// fmt.Println("n:", name, n, tryVal, v, surl, match, err)
				if err != nil {
					if strings.Contains(name, "*") {
						wildError = err
						continue
					}
					errs = append(errs, err)
				}
				continue outer
			}
		}
		err := wildError
		if err == nil {
			err = zlog.NewError("Name Value Not Found:", name, tryVal)
		}
		errs = append(errs, err)
	}
	return
}
