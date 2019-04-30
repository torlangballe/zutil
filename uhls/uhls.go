package uhls

import (
	"bufio"
	"math/rand"
	"net/http"
	"net/url"
	"path"

	"githubclones/grafov/m3u8"

	"github.com/pkg/errors"
)

type Master struct {
	Url      string
	Variants []*Variant
}

type Variant struct {
	Url        string
	SequenceId int
	BitsPerSec int64
	Duration   float64
	Segments   []Segment
}

type Segment struct {
	Duration float64
	Url      string
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

func ReadFromUrl(surl string) (m *Master, err error) {
	master, err := getMasterPlaylist(surl)
	if err != nil {
		return
	}
	m = &Master{}
	m.Url = surl
	for _, mv := range master.Variants {
		v := &Variant{}
		v.SequenceId = int(mv.ProgramId)
		v.BitsPerSec = int64(mv.Bandwidth)
		v.Url = replaceUrlNameWithPath(surl, mv.URI)
		plist, e := getMediaPlaylist(v.Url)
		if e != nil {
			err = e
			return
		}
		for _, ms := range plist.Segments {
			if ms == nil { // due to bug in github.com/grafov/m3u8, must fix
				continue
			}
			var s Segment
			s.Url = replaceUrlNameWithPath(v.Url, ms.URI)
			s.Duration = ms.Duration
			v.Duration += s.Duration
			v.Segments = append(v.Segments, s)
		}
		m.Variants = append(m.Variants, v)
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
	strict := false
	p, listType, err := m3u8.DecodeFrom(bufio.NewReader(resp.Body), strict)
	if err != nil {
		return
	}
	if listType == m3u8.MEDIA {
		plist = p.(*m3u8.MediaPlaylist)
	} else {
		err = errors.New("Wrong playlist type for media")
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
			seg = mv.Segments[i]
			return
		}
		i -= len(mv.Segments)
	}
	return
}
