package zsmoothstream

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/torlangballe/zutil/zbytes"

	"github.com/torlangballe/zutil/uhttp"
	"github.com/torlangballe/zutil/uxml"
)

// https://johnnyshao.wordpress.com/2011/04/06/implement-a-smooth-streaming-client-1-basic-knowledges/
// https://www.iis.net/downloads/microsoft/smooth-streaming
// https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-sstr/8383f27f-7efe-4c60-832a-387274457251

type SmoothStream struct {
	//	XMLNS                *string `xml:"xmlns,attr"`
	//	SmoothStreamingMedia struct {
	Duration    string `xml:"Duration,attr"`
	StreamIndex []struct {
		Type         string `xml:"Type,attr"`
		Url          string `xml:"Url,attr"`
		QualityLevel []struct {
			Index     string `xml:"Index,attr"`
			Bitrate   string `xml:"Bitrate,attr"`
			FourCC    string `xml:"FourCC,attr"`
			MaxWidth  string `xml:"MaxWidth,attr"`
			MaxHeight string `xml:"MaxHeight,attr"`
		} `xml:"QualityLevel"`
		C []struct {
			Duration string `xml:"d,attr"`
		} `xml:"c"`
	} `xml:"StreamIndex"`
	DurationSecs float64
	RawBytes     []byte
}

type SimpleProfile struct {
	Type    string
	URL     string
	Bitrate int
	Codec   string
	Width   int
	Height  int
	Chunks  []Chunk
}

type Chunk struct {
	Pos float64
	URL string
}

func GetFromUrl(surl string) (ss *SmoothStream, err error) {
	// surl, err = uhttp.GetRedirectedURL(surl)
	// if err != nil {
	// 	return
	// }
	resp, err := http.Get(surl)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	ss = new(SmoothStream)
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}
	ss.RawBytes = body
	if zbytes.HasUnicodeBOM(body) {
		ss.RawBytes, _ = zbytes.DecodeUTF16(body)
	}
	fmt.Println("SSRAW:", err, string(ss.RawBytes[:200]))
	err = uxml.UnmarshalWithBOM(body, ss)
	if err != nil {
		return
	}
	d, _ := strconv.ParseInt(ss.Duration, 10, 64)
	ss.DurationSecs = float64(d) / 1000000
	return
}

func (ss *SmoothStream) GetSimpleProfiles(surl string) (profiles []SimpleProfile) {
	surl, _ = uhttp.GetRedirectedURL(surl)
	urlPrefix, _ := path.Split(surl)
	for _, s := range ss.StreamIndex {
		var sp SimpleProfile
		sp.Type = s.Type
		chunkURLAdd := s.Url
		for _, q := range s.QualityLevel {
			p := sp
			p.Bitrate, _ = strconv.Atoi(q.Bitrate)
			p.Codec = q.FourCC
			p.Width, _ = strconv.Atoi(q.MaxWidth)
			p.Height, _ = strconv.Atoi(q.MaxHeight)
			p.URL = urlPrefix
			chunkURLAdd = strings.Replace(chunkURLAdd, "{bitrate}", q.Bitrate, -1)
			var pos int64
			for _, c := range s.C {
				var chunk Chunk
				chunk.Pos = float64(pos) / 1000000
				u := strings.Replace(chunkURLAdd, "{start time}", fmt.Sprintf("%d", pos), -1)
				chunk.URL = uhttp.AddPathToURL(urlPrefix, u)
				d, _ := strconv.ParseInt(c.Duration, 10, 64)
				pos += d
				p.Chunks = append(p.Chunks, chunk)
			}
			profiles = append(profiles, p)
		}
	}
	return
}
