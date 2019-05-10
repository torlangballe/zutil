package uhls

import (
	"bufio"
	"capsulefm/libs/util/uhttp"
	"math/rand"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"github.com/torlangballe/zutil/uhtml"
	"github.com/torlangballe/zutil/ustr"

	"githubclones/grafov/m3u8"

	"github.com/pkg/errors"
)

type Master struct {
	URL      string
	Variants []*Variant
}

type Variant struct {
	URL          string
	SequenceId   int
	BitsPerSec   int64
	Resolution   string
	FrameRate    float64
	Codecs       string
	Duration     float64
	Segments     []*Segment
	SegmentCount int // this is set even if segments not read
	Error        error
}

type Segment struct {
	Duration float64
	URL      string
	Title    string
	SeqId    uint64
	Limit    int64 // EXT-X-BYTERANGE <n> is length in bytes for the file under URI
	Offset   int64 // EXT-X-BYTERANGE [@o] is offset from the start of the file under URI
}

func replaceUrlNameWithPath(surl, spath string) string {
	u, err := url.Parse(surl)
	if err != nil {
		return surl
	}
	dir, _ := path.Split(u.Path)
	u.Path = path.Join(dir, spath)
	return u.String()
}

func getSegments(wg *sync.WaitGroup, v *Variant, surl string) {
	v.Segments, v.Error = GetSegmentsFromUrl(surl)
	for _, s := range v.Segments {
		v.Duration += s.Duration
	}
	wg.Done()
}

func ReadFromUrl(surl string, getSegs bool) (m *Master, err error) {
	master, err := getMasterPlaylist(surl)
	if err != nil {
		return
	}
	m = &Master{}
	m.URL = surl
	wg := new(sync.WaitGroup)
	for _, mv := range master.Variants {
		v := &Variant{}
		v.SequenceId = int(mv.ProgramId)
		v.BitsPerSec = int64(mv.Bandwidth)
		v.URL = replaceUrlNameWithPath(surl, mv.URI)
		v.Resolution = mv.Resolution
		v.FrameRate = mv.FrameRate
		v.Codecs = mv.VariantParams.Codecs
		if getSegs {
			wg.Add(1)
			go getSegments(wg, v, v.URL)
		}
		m.Variants = append(m.Variants, v)
	}
	wg.Wait()

	return
}

func GetSegmentsFromUrl(surl string) (segs []*Segment, err error) {
	plist, e := getMediaPlaylist(surl)
	if e != nil {
		err = e
		return
	}
	for i, ms := range plist.Segments {
		if i >= int(plist.Count()) {
			break
		}
		if ms == nil { // due to bug in github.com/grafov/m3u8, must fix
			continue
		}
		var s Segment
		s.URL = replaceUrlNameWithPath(surl, ms.URI)
		s.Duration = ms.Duration
		t := strings.TrimSpace(ms.Title)
		if t != "no desc" {
			s.Title = t
		}
		s.SeqId = ms.SeqId
		s.Limit = ms.Limit
		s.Offset = ms.Offset

		segs = append(segs, &s)
	}
	return
}

func getMasterPlaylist(url string) (plist *m3u8.MasterPlaylist, err error) {
	resp, err := http.Get(url)
	if err != nil {
		return
	}
	if resp.StatusCode >= 400 {
		err = errors.Errorf("Error getting: %d", resp.StatusCode)
		return
	}
	strict := false
	p, listType, err := m3u8.DecodeFrom(bufio.NewReader(resp.Body), strict)
	if err != nil {
		return
	}
	if listType == m3u8.MASTER {
		plist = p.(*m3u8.MasterPlaylist)
	} else {
		err = errors.New("Wrong playlist type for master")
	}
	return
}

func getMediaPlaylist(url string) (plist *m3u8.MediaPlaylist, err error) {
	resp, err := http.Get(url)
	if err != nil {
		return
	}
	mtype, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if mtype == "text/html" {
		sbody := uhttp.GetCopyOfResponseBodyAsString(resp)
		text, _ := uhtml.ExtractTextFromHTMLString(sbody)
		if text != "" {
			err = errors.New(ustr.HeadUntilString(text, "\n"))
		} else {
			err = errors.New("reponse was error message")
		}
		return
	}
	strict := false
	p, listType, err := m3u8.DecodeFrom(bufio.NewReader(resp.Body), strict)
	if err != nil {
		return
	}
	if listType == m3u8.MASTER {
		err = errors.New("Wrong playlist type for media")
	} else {
		plist = p.(*m3u8.MediaPlaylist)
	}
	return
}

func (m *Master) GetRandomSegment() (seg Segment, v Variant) {
	count := 0
	for _, mv := range m.Variants {
		count += len(mv.Segments)
	}
	i := int(rand.Int31n(int32(count)))
	for _, mv := range m.Variants {
		if i < len(mv.Segments) {
			v = *mv
			seg = *mv.Segments[i]
			return
		}
		i -= len(mv.Segments)
	}
	return
}
