package uimages

import (
	"zutil/uhttp"
	"fmt"
	"net/url"
	"strconv"
)

const (
	KImaggaApiKey = "acc_f754c8bd63dcf64"
)

type Tag struct {
	Name       string `json:"name"`
	Confidence string `json:"confidence"`
}

type Info struct {
	Tags []Tag `json:"tags"`
}

type Category struct {
	Name       string
	Confidence float64
}

var categoryMap = map[string]*[]Category{}

func EvaluateImageToCategories(simage string, max int) (cats []Category, err error) {

	m := categoryMap[simage]
	if m != nil {
		fmt.Println("Using cache for image url:", simage)
		cats = *m
		return
	}
	var info []Info
	args := map[string]string{
		"api_key": KImaggaApiKey,
	}
	values := url.Values{"urls": {simage}}

	//	sbase := "http://api.imagga.com/draft/classify/capsule_fm_test_v2"
	sbase := "http://api.imagga.com/draft/classify/mobile_photos_sliki_v7"
	surl, err := uhttp.MakeURLWithArgs(sbase, args)
	//	fmt.Println("Imagga URL:", simage)
	if err != nil {
		fmt.Println("EvaluateImageToCategories: makeurl:", err)
		return
	}
	err = uhttp.UnmarshalFromJSONFromPostForm(surl, values, &info, false)
	if err != nil {
		fmt.Println("EvaluateImageToCategories: unmarshal:", err)
		return
	}
	if len(info) > 0 {
		for i, t := range info[0].Tags {
			var c Category

			c.Name = t.Name
			c.Confidence, _ = strconv.ParseFloat(t.Confidence, 64)
			cats = append(cats, c)
			if i == 1 {
				break
			}
		}
	}
	return
}
